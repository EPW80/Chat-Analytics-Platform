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
