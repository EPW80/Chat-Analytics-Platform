import { useEffect, useMemo, useRef, useState } from 'react';
import { AnalyticsDashboard } from './components/AnalyticsDashboard';
import { MessageInput } from './components/MessageInput';
import { MessageList } from './components/MessageList';
import { UserList } from './components/UserList';
import { useAnalytics } from './hooks/useAnalytics';
import { useWebSocket } from './hooks/useWebSocket';

const WS_URL = import.meta.env.VITE_WS_URL ?? 'ws://localhost:8080/ws';

function generateUserId(): string {
  return 'user-' + Math.random().toString(36).slice(2, 10);
}

type Tab = 'chat' | 'analytics';

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

  const statusColor: Record<typeof connectionStatus, string> = {
    connected: 'bg-green-400',
    connecting: 'bg-yellow-400 animate-pulse',
    disconnected: 'bg-red-400',
    failed: 'bg-red-600',
  };

  if (!confirmed) {
    return (
      <div className="min-h-screen bg-gray-950 flex items-center justify-center">
        <div className="bg-gray-900 rounded-xl p-8 w-80 space-y-4 shadow-xl">
          <h1 className="text-xl font-bold text-white text-center">Chat Analytics</h1>
          <p className="text-sm text-gray-400 text-center">Enter a display name to join</p>
          <input
            ref={inputRef}
            type="text"
            maxLength={50}
            placeholder="Your name…"
            value={username}
            onChange={(e) => setUsername(e.target.value)}
            onKeyDown={(e) => { if (e.key === 'Enter' && username.trim()) setConfirmed(true); }}
            className="w-full bg-gray-800 text-white rounded px-3 py-2 text-sm focus:outline-none focus:ring-1 focus:ring-indigo-500"
          />
          <button
            disabled={!username.trim()}
            onClick={() => setConfirmed(true)}
            className="w-full py-2 rounded bg-indigo-600 text-white text-sm font-medium hover:bg-indigo-500 disabled:opacity-40"
          >
            Join
          </button>
        </div>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-gray-950 text-white flex flex-col">
      {/* Header */}
      <header className="border-b border-gray-800 bg-gray-900 px-4 py-3 flex items-center gap-4">
        <h1 className="font-bold text-lg text-indigo-400">Chat Analytics</h1>
        <span className="text-gray-500 text-sm">@{username}</span>
        <div className="flex items-center gap-1.5 ml-auto">
          <span className={`w-2 h-2 rounded-full ${statusColor[connectionStatus]}`} />
          <span className="text-xs text-gray-400 capitalize">{connectionStatus}</span>
        </div>
        <nav className="flex gap-1 ml-4">
          {(['chat', 'analytics'] as Tab[]).map((t) => (
            <button
              key={t}
              onClick={() => setTab(t)}
              className={`px-3 py-1 rounded text-sm font-medium capitalize transition-colors ${
                tab === t ? 'bg-indigo-600 text-white' : 'text-gray-400 hover:text-white'
              }`}
            >
              {t}
            </button>
          ))}
        </nav>
      </header>

      {tab === 'chat' && (
        <div className="flex flex-1 overflow-hidden">
          <div className="flex flex-col flex-1 overflow-hidden">
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
