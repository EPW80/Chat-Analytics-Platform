import { useEffect, useMemo, useRef, useState } from 'react';
import { Activity, BarChart3, MessageSquare } from 'lucide-react';
import { AnalyticsDashboard } from './components/AnalyticsDashboard';
import { MessageInput } from './components/MessageInput';
import { MessageList } from './components/MessageList';
import { UserList } from './components/UserList';
import { Badge } from './components/ui/Badge';
import { Button } from './components/ui/Button';
import { Card } from './components/ui/Card';
import { Input } from './components/ui/Input';
import { StatusDot } from './components/ui/StatusDot';
import { useAnalytics } from './hooks/useAnalytics';
import { useWebSocket } from './hooks/useWebSocket';
import { cn } from './lib/cn';

const WS_URL = import.meta.env.VITE_WS_URL ?? 'ws://localhost:8080/ws';

function generateUserId(): string {
  return 'user-' + Math.random().toString(36).slice(2, 10);
}

type Tab = 'chat' | 'analytics';
type ConnectionStatus = 'connected' | 'connecting' | 'disconnected' | 'failed';

const statusTone: Record<ConnectionStatus, 'success' | 'warning' | 'danger'> = {
  connected: 'success',
  connecting: 'warning',
  disconnected: 'danger',
  failed: 'danger',
};

const TABS: { id: Tab; label: string; Icon: typeof MessageSquare }[] = [
  { id: 'chat', label: 'Chat', Icon: MessageSquare },
  { id: 'analytics', label: 'Analytics', Icon: BarChart3 },
];

export default function App() {
  const [username, setUsername] = useState('');
  const [confirmed, setConfirmed] = useState(false);
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => { inputRef.current?.focus(); }, []);

  const userId = useMemo(() => generateUserId(), []);
  const wsUrl = confirmed ? `${WS_URL}?userId=${userId}&username=${encodeURIComponent(username)}` : '';

  const { messages, sendMessage, isConnected, connectionStatus } = useWebSocket(wsUrl);
  const { metrics } = useAnalytics(5000);

  const [tab, setTab] = useState<Tab>('chat');

  const activeUsers = metrics?.activeUserDetails ?? [];

  if (!confirmed) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-bg p-4">
        <Card padding="lg" className="w-full max-w-sm shadow-panel">
          <div className="mb-6 flex flex-col items-center gap-3 text-center">
            <div className="flex h-12 w-12 items-center justify-center rounded-xl bg-accent/15 text-accent">
              <Activity className="h-6 w-6" />
            </div>
            <div>
              <h1 className="text-xl font-bold text-text">Chat Analytics</h1>
              <p className="mt-1 text-sm text-muted">Enter a display name to join</p>
            </div>
          </div>
          <div className="space-y-3">
            <Input
              ref={inputRef}
              type="text"
              maxLength={50}
              placeholder="Your name…"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              onKeyDown={(e) => { if (e.key === 'Enter' && username.trim()) setConfirmed(true); }}
            />
            <Button
              disabled={!username.trim()}
              onClick={() => setConfirmed(true)}
              className="w-full"
            >
              Join chat
            </Button>
          </div>
        </Card>
      </div>
    );
  }

  return (
    <div className="flex min-h-screen flex-col bg-bg text-text">
      <header className="flex items-center gap-3 border-b border-border bg-surface px-4 py-3">
        <div className="flex items-center gap-2">
          <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-accent/15 text-accent">
            <Activity className="h-5 w-5" />
          </div>
          <h1 className="text-lg font-bold text-text">Chat Analytics</h1>
        </div>
        <span className="hidden text-sm text-subtle sm:inline">@{username}</span>

        <Badge tone={statusTone[connectionStatus]} className="ml-auto capitalize">
          <StatusDot tone={statusTone[connectionStatus]} pulse={connectionStatus === 'connecting'} />
          {connectionStatus}
        </Badge>

        <nav className="flex gap-1 rounded-lg bg-surface-raised p-1">
          {TABS.map(({ id, label, Icon }) => (
            <button
              key={id}
              onClick={() => setTab(id)}
              className={cn(
                'inline-flex items-center gap-1.5 rounded-md px-3 py-1.5 text-sm font-medium transition-colors',
                tab === id ? 'bg-accent text-accent-fg' : 'text-muted hover:text-text',
              )}
            >
              <Icon className="h-4 w-4" />
              <span className="hidden sm:inline">{label}</span>
            </button>
          ))}
        </nav>
      </header>

      {tab === 'chat' && (
        <div className="flex flex-1 overflow-hidden">
          <div className="flex flex-1 flex-col overflow-hidden">
            <MessageList messages={messages} />
            <MessageInput onSend={sendMessage} disabled={!isConnected} />
          </div>
          <UserList users={activeUsers} />
        </div>
      )}

      {tab === 'analytics' && (
        <div className="flex flex-1 overflow-hidden">
          <AnalyticsDashboard metrics={metrics} />
        </div>
      )}
    </div>
  );
}
