package analytics

import "time"

// AnomalyKind enumerates the traffic conditions the detector recognizes.
type AnomalyKind string

const (
	RateSpike       AnomalyKind = "rate_spike"
	LatencyBreach   AnomalyKind = "latency_breach"
	ConnectionSurge AnomalyKind = "connection_surge"
	TrafficDropout  AnomalyKind = "traffic_dropout"
)

// Severity bands. Yellow = worth a briefing; Red = worth a briefing with urgency.
type Severity string

const (
	Yellow Severity = "yellow"
	Red    Severity = "red"
)

// Anomaly is one detected condition. It is the sole hand-off contract from the
// analytics domain to the briefing domain: briefing.Compose takes an Anomaly and
// nothing else, so everything the narration needs must live here.
type Anomaly struct {
	Kind     AnomalyKind
	Scope    string  // "global" for v1; room ID once per-room detection lands
	Observed float64 // the value that tripped the rule
	Baseline float64 // what it was compared against (0 when N/A, e.g. dropout)
	Severity Severity
	At       time.Time
}

// Thresholds are the tunable knobs. Defaults are Opus-tier decisions to be
// calibrated against real traffic — see DetectorConfig defaults below.
type Thresholds struct {
	SpikeMultiplier float64 // current > baseline*K to fire a RateSpike
	SpikeFloor      int64   // and current must exceed this (kills idle-server noise)
	LatencyYellowMs float64 // P99 ceiling for Yellow
	LatencyRedMs    float64 // P99 ceiling for Red
	ConnSurgeDelta  int64   // ActiveConnections jump vs previous snapshot
	DebounceWindow  time.Duration
}

// DefaultThresholds — placeholders. Tune against real traffic before the demo.
func DefaultThresholds() Thresholds {
	return Thresholds{
		SpikeMultiplier: 3.0,
		SpikeFloor:      50,
		LatencyYellowMs: 50,
		LatencyRedMs:    150,
		ConnSurgeDelta:  100,
		DebounceWindow:  5 * time.Minute,
	}
}

// Detector is stateful: it remembers the previous snapshot (for surge/dropout
// deltas) and the last time each (Kind,Scope) fired (for debounce). It performs
// NO I/O and does not read the clock implicitly — callers pass `now`. This keeps
// Check a pure function of its inputs, so tests are millisecond-fast and CI never
// touches the network.
type Detector struct {
	th       Thresholds
	prev     *Metrics // nil until the first Check
	lastFire map[fireKey]time.Time
}

type fireKey struct {
	kind  AnomalyKind
	scope string
}

// NewDetector returns a ready detector. Pass DefaultThresholds() unless overriding.
func NewDetector(th Thresholds) *Detector {
	return &Detector{th: th, lastFire: make(map[fireKey]time.Time)}
}

// scopeGlobal is the only scope in v1 — the Tracker is a single global instance,
// so every anomaly spans all rooms. Per-room detection is a documented stretch.
const scopeGlobal = "global"

// Check compares the current snapshot against the previous one and returns the
// anomalies that (a) trip a rule and (b) are outside their debounce window.
// It updates internal state (prev + lastFire) as a side effect on the receiver.
//
// Check is pure over its inputs — no I/O, and `now` is passed rather than read —
// so tests are millisecond-fast and CI never touches the network.
func (d *Detector) Check(cur Metrics, now time.Time) []Anomaly {
	var out []Anomaly

	curRate, baseline, haveWindow := rateWindow(cur.MessagesPerMinute)

	// RateSpike — current minute far above the trailing baseline. The floor guard
	// matters more than the multiplier: on an idle server a baseline near zero
	// makes any blip an infinite multiple.
	if haveWindow && curRate > d.th.SpikeFloor && float64(curRate) > baseline*d.th.SpikeMultiplier {
		sev := Yellow
		if float64(curRate) > baseline*d.th.SpikeMultiplier*2 {
			sev = Red
		}
		if d.shouldFire(RateSpike, scopeGlobal, now) {
			out = append(out, Anomaly{
				Kind:     RateSpike,
				Scope:    scopeGlobal,
				Observed: float64(curRate),
				Baseline: baseline,
				Severity: sev,
				At:       now,
			})
		}
	}

	// LatencyBreach — P99 over an absolute ceiling. Red wins over Yellow; only
	// one latency anomaly per check.
	if cur.LatencyP99Ms > d.th.LatencyYellowMs {
		sev := Yellow
		threshold := d.th.LatencyYellowMs
		if cur.LatencyP99Ms > d.th.LatencyRedMs {
			sev = Red
			threshold = d.th.LatencyRedMs
		}
		if d.shouldFire(LatencyBreach, scopeGlobal, now) {
			out = append(out, Anomaly{
				Kind:     LatencyBreach,
				Scope:    scopeGlobal,
				Observed: cur.LatencyP99Ms,
				Baseline: threshold,
				Severity: sev,
				At:       now,
			})
		}
	}

	// Surge and dropout are deltas, so they need a previous snapshot. The first
	// Check after startup establishes the baseline and reports neither.
	if d.prev != nil {
		delta := cur.ActiveConnections - d.prev.ActiveConnections
		if delta > d.th.ConnSurgeDelta {
			sev := Yellow
			if delta > d.th.ConnSurgeDelta*2 {
				sev = Red
			}
			if d.shouldFire(ConnectionSurge, scopeGlobal, now) {
				out = append(out, Anomaly{
					Kind:     ConnectionSurge,
					Scope:    scopeGlobal,
					Observed: float64(cur.ActiveConnections),
					Baseline: float64(d.prev.ActiveConnections),
					Severity: sev,
					At:       now,
				})
			}
		}

		// TrafficDropout — the room was talking last poll and has gone silent.
		// Baseline is 0 (N/A): the previous rate is the Observed-side context.
		prevRate, _, prevWindow := rateWindow(d.prev.MessagesPerMinute)
		if prevWindow && haveWindow && prevRate > 0 && curRate == 0 {
			if d.shouldFire(TrafficDropout, scopeGlobal, now) {
				out = append(out, Anomaly{
					Kind:     TrafficDropout,
					Scope:    scopeGlobal,
					Observed: float64(prevRate),
					Baseline: 0,
					Severity: Yellow,
					At:       now,
				})
			}
		}
	}

	// Copy the window: Metrics carries a slice, and retaining the caller's array
	// would let a later mutation rewrite our baseline out from under us.
	snapshot := cur
	snapshot.MessagesPerMinute = append([]int64(nil), cur.MessagesPerMinute...)
	d.prev = &snapshot

	return out
}

// rateWindow splits a MessagesPerMinute window into the most recent COMPLETE
// minute and the mean of the minutes before it. The final slot is the minute
// currently in progress: it is a partial count that resets on every rotation, so
// comparing it against full-minute baselines reports a dropout at the top of
// every minute on a busy server. ok is false when the window is too short to hold
// a complete minute plus a baseline.
//
// The cost of ignoring the partial slot is granularity: an event is only visible
// once the minute containing it has closed, so detection lags by up to a minute.
func rateWindow(perMin []int64) (current int64, baseline float64, ok bool) {
	if len(perMin) < 3 {
		return 0, 0, false
	}
	complete := perMin[:len(perMin)-1] // drop the in-progress slot
	current = complete[len(complete)-1]
	var sum int64
	for _, v := range complete[:len(complete)-1] {
		sum += v
	}
	baseline = float64(sum) / float64(len(complete)-1)
	return current, baseline, true
}

// shouldFire reports whether (kind,scope) is outside its debounce window at `now`,
// and records the fire time when it returns true. Without this an oscillating
// metric would generate a briefing — and an ElevenLabs call — every tick.
func (d *Detector) shouldFire(kind AnomalyKind, scope string, now time.Time) bool {
	k := fireKey{kind: kind, scope: scope}
	if last, seen := d.lastFire[k]; seen && now.Sub(last) < d.th.DebounceWindow {
		return false
	}
	d.lastFire[k] = now
	return true
}
