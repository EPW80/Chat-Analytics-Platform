export type MessageType = 'chat' | 'system' | 'join' | 'leave';

export interface Message {
  messageId: string;
  roomId: string;
  type: MessageType;
  userId: string;
  username: string;
  content: string;
  timestamp: string;
}

export interface UserInfo {
  userId: string;
  username: string;
  joinedAt: string;
}

export type AnomalyKind = 'rate_spike' | 'latency_breach' | 'connection_surge' | 'traffic_dropout';

export type Severity = 'yellow' | 'red';

/** One voice briefing. Mirrors briefing.Record on the backend. */
export interface Briefing {
  hash: string;
  kind: AnomalyKind;
  scope: string;
  severity: Severity;
  script: string;
  /** Empty string means text-only: TTS failed or the character budget was spent. */
  audioUrl: string;
  createdAt: string;
}

/**
 * Frames arriving over the socket. The backend batches newline-delimited JSON,
 * and `type` is the discriminator — a chat message and a briefing have nothing
 * else in common, so this must be a union rather than a cast.
 */
export type InboundFrame = Message | { type: 'briefing.ready'; briefing: Briefing };

export interface AnalyticsMetrics {
  totalMessages: number;
  activeUsers: number;
  peakConnections: number;
  messagesPerMinute: number[];
  latencyP50Ms: number;
  latencyP95Ms: number;
  latencyP99Ms: number;
  activeUserDetails: UserInfo[];
  uptimeSeconds: number;
  serverStartTime: string;
}
