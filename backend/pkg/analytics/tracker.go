package analytics

import (
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/epw80/chat-analytics-platform/pkg/message"
)

const maxLatencySamples = 1000

// Tracker collects real-time analytics events using atomic counters and
// in-memory structures. All methods are safe for concurrent use.
//
// Connections and users are tracked separately: a single user (userID) may
// hold several concurrent connections (e.g. multiple browser tabs). The
// connections map is keyed by the per-connection client ID, while userRefs
// ref-counts active connections per user so a tab closing doesn't drop a user
// who is still connected elsewhere.
type Tracker struct {
	totalMessages   atomic.Int64
	peakConnections atomic.Int64

	mu             sync.RWMutex
	connections    map[string]UserInfo // keyed by clientID
	userRefs       map[string]int      // userID -> active connection count
	latencySamples []time.Duration     // ring buffer capped at maxLatencySamples
	window         *slidingWindow
	startTime      time.Time
}

// New creates a ready-to-use Tracker.
func New() *Tracker {
	return &Tracker{
		connections: make(map[string]UserInfo),
		userRefs:    make(map[string]int),
		window:      newWindow(),
		startTime:   time.Now(),
	}
}

// TrackMessage records an inbound chat message.
func (t *Tracker) TrackMessage(msg *message.Message) {
	t.totalMessages.Add(1)
	t.window.increment()
}

// TrackConnect records a new connection for the given client and user.
func (t *Tracker) TrackConnect(clientID, userID, username string) {
	t.mu.Lock()
	t.connections[clientID] = UserInfo{
		ClientID: clientID,
		UserID:   userID,
		Username: username,
		JoinedAt: time.Now(),
	}
	t.userRefs[userID]++
	count := int64(len(t.connections))
	t.mu.Unlock()

	// Update peak connections if this is a new high.
	for {
		peak := t.peakConnections.Load()
		if count <= peak {
			break
		}
		if t.peakConnections.CompareAndSwap(peak, count) {
			break
		}
	}
}

// TrackDisconnect records a disconnection for the given client and user.
func (t *Tracker) TrackDisconnect(clientID, userID string) {
	t.mu.Lock()
	delete(t.connections, clientID)
	if t.userRefs[userID] <= 1 {
		delete(t.userRefs, userID)
	} else {
		t.userRefs[userID]--
	}
	t.mu.Unlock()
}

// TrackBroadcastLatency records how long a broadcast fan-out took.
func (t *Tracker) TrackBroadcastLatency(d time.Duration) {
	t.mu.Lock()
	if len(t.latencySamples) >= maxLatencySamples {
		// Evict oldest sample.
		t.latencySamples = t.latencySamples[1:]
	}
	t.latencySamples = append(t.latencySamples, d)
	t.mu.Unlock()
}

// GetMetrics returns a consistent point-in-time snapshot.
func (t *Tracker) GetMetrics() Metrics {
	t.mu.RLock()
	users := make([]UserInfo, 0, len(t.connections))
	for _, u := range t.connections {
		users = append(users, u)
	}
	activeConnections := int64(len(t.connections))
	activeUsers := int64(len(t.userRefs))
	p50, p95, p99 := calcPercentiles(t.latencySamples)
	t.mu.RUnlock()

	return Metrics{
		TotalMessages:     t.totalMessages.Load(),
		ActiveConnections: activeConnections,
		ActiveUsers:       activeUsers,
		PeakConnections:   t.peakConnections.Load(),
		MessagesPerMinute: t.window.snapshot(),
		LatencyP50Ms:      p50,
		LatencyP95Ms:      p95,
		LatencyP99Ms:      p99,
		ActiveUserDetails: users,
		UptimeSeconds:     int64(time.Since(t.startTime).Seconds()),
		ServerStartTime:   t.startTime,
	}
}

// calcPercentiles returns P50/P95/P99 in milliseconds from a sample slice.
// samples must not be modified while this runs (caller holds the read lock).
func calcPercentiles(samples []time.Duration) (p50, p95, p99 float64) {
	if len(samples) == 0 {
		return 0, 0, 0
	}
	sorted := make([]time.Duration, len(samples))
	copy(sorted, samples)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

	toMs := func(d time.Duration) float64 { return float64(d.Microseconds()) / 1000.0 }
	idx := func(pct float64) int {
		i := int(pct * float64(len(sorted)-1))
		if i >= len(sorted) {
			i = len(sorted) - 1
		}
		return i
	}
	return toMs(sorted[idx(0.50)]), toMs(sorted[idx(0.95)]), toMs(sorted[idx(0.99)])
}
