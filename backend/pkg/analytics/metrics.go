package analytics

import "time"

// Metrics is a point-in-time snapshot of analytics data.
type Metrics struct {
	TotalMessages     int64      `json:"totalMessages"`
	ActiveUsers       int64      `json:"activeUsers"`
	PeakConnections   int64      `json:"peakConnections"`
	MessagesPerMinute []int64    `json:"messagesPerMinute"` // last 15 minutes, oldest first
	LatencyP50Ms      float64    `json:"latencyP50Ms"`
	LatencyP95Ms      float64    `json:"latencyP95Ms"`
	LatencyP99Ms      float64    `json:"latencyP99Ms"`
	ActiveUserDetails []UserInfo `json:"activeUserDetails"`
	UptimeSeconds     int64      `json:"uptimeSeconds"`
	ServerStartTime   time.Time  `json:"serverStartTime"`
}

// UserInfo holds display info for a connected user.
type UserInfo struct {
	UserID   string    `json:"userId"`
	Username string    `json:"username"`
	JoinedAt time.Time `json:"joinedAt"`
}
