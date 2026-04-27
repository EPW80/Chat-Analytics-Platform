import { useEffect, useRef } from 'react';
import { FixedSizeList, type ListChildComponentProps } from 'react-window';
import type { Message } from '../types/message';

interface Props {
  messages: Message[];
}

const ROW_HEIGHT = 64;

function Row({ index, style, data }: ListChildComponentProps<Message[]>) {
  const msg = data[index];
  const time = new Date(msg.timestamp).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });

  if (msg.type === 'system' || msg.type === 'join' || msg.type === 'leave') {
    return (
      <div style={style} className="flex items-center justify-center px-4">
        <span className="text-xs text-gray-500 italic">{msg.content || `${msg.username} ${msg.type}ed`}</span>
      </div>
    );
  }

  const color = userColor(msg.userId);
  return (
    <div style={style} className="flex flex-col justify-center px-4 py-1 hover:bg-gray-800/40">
      <div className="flex items-baseline gap-2">
        <span className="text-sm font-semibold" style={{ color }}>{msg.username}</span>
        <span className="text-xs text-gray-500">{time}</span>
      </div>
      <p className="text-sm text-gray-200 break-words leading-snug">{msg.content}</p>
    </div>
  );
}

function userColor(userId: string): string {
  const colors = ['#60a5fa', '#34d399', '#f472b6', '#fb923c', '#a78bfa', '#38bdf8', '#4ade80'];
  let hash = 0;
  for (let i = 0; i < userId.length; i++) hash = (hash * 31 + userId.charCodeAt(i)) & 0xffffffff;
  return colors[Math.abs(hash) % colors.length];
}

export function MessageList({ messages }: Props) {
  const listRef = useRef<FixedSizeList>(null);

  useEffect(() => {
    if (messages.length > 0) {
      listRef.current?.scrollToItem(messages.length - 1, 'end');
    }
  }, [messages.length]);

  if (messages.length === 0) {
    return (
      <div className="flex-1 flex items-center justify-center text-gray-500 text-sm">
        No messages yet. Say hello!
      </div>
    );
  }

  return (
    <div className="flex-1 overflow-hidden">
      <FixedSizeList
        ref={listRef}
        height={600}
        itemCount={messages.length}
        itemSize={ROW_HEIGHT}
        itemData={messages}
        width="100%"
        className="scrollbar-thin"
      >
        {Row}
      </FixedSizeList>
    </div>
  );
}
