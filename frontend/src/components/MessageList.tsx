import { useEffect } from 'react';
import { MessagesSquare } from 'lucide-react';
import { List, useListRef, type RowComponentProps } from 'react-window';
import type { Message } from '../types/message';
import { initials, userColor } from '../lib/userColor';

interface Props {
  messages: Message[];
}

const ROW_HEIGHT = 64;

function Row({
  index,
  style,
  messages,
  ariaAttributes,
}: RowComponentProps<{ messages: Message[] }>) {
  const msg = messages[index];
  if (!msg) return null;

  const time = new Date(msg.timestamp).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });

  if (msg.type === 'system' || msg.type === 'join' || msg.type === 'leave') {
    return (
      <div {...ariaAttributes} style={style} className="flex items-center justify-center px-4">
        <span className="rounded-full bg-surface-raised px-3 py-1 text-xs text-subtle">
          {msg.content || `${msg.username} ${msg.type}ed`}
        </span>
      </div>
    );
  }

  const color = userColor(msg.userId);
  return (
    <div
      {...ariaAttributes}
      style={style}
      className="flex items-start gap-3 px-4 py-1.5 transition-colors hover:bg-surface-raised"
    >
      <div
        className="mt-0.5 flex h-9 w-9 shrink-0 items-center justify-center rounded-full text-xs font-semibold text-bg"
        style={{ backgroundColor: color }}
        aria-hidden
      >
        {initials(msg.username)}
      </div>
      <div className="min-w-0 flex-1">
        <div className="flex items-baseline gap-2">
          <span className="text-sm font-semibold" style={{ color }}>{msg.username}</span>
          <span className="text-xs text-subtle">{time}</span>
        </div>
        <p className="break-words text-sm leading-snug text-text/90">{msg.content}</p>
      </div>
    </div>
  );
}

export function MessageList({ messages }: Props) {
  const listRef = useListRef(null);

  useEffect(() => {
    if (messages.length === 0) return;
    listRef.current?.scrollToRow({ index: messages.length - 1, align: 'end' });
  }, [messages.length, listRef]);

  if (messages.length === 0) {
    return (
      <div className="flex flex-1 flex-col items-center justify-center gap-3 text-subtle">
        <MessagesSquare className="h-10 w-10 opacity-40" />
        <p className="text-sm">No messages yet. Say hello!</p>
      </div>
    );
  }

  return (
    <List
      listRef={listRef}
      className="flex-1 scrollbar-thin"
      style={{ height: '100%' }}
      rowComponent={Row}
      rowCount={messages.length}
      rowHeight={ROW_HEIGHT}
      rowProps={{ messages }}
    />
  );
}
