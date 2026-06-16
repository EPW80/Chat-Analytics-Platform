import { Users } from 'lucide-react';
import type { UserInfo } from '../types/message';
import { Badge } from './ui/Badge';
import { StatusDot } from './ui/StatusDot';
import { initials, userColor } from '../lib/userColor';

interface Props {
  users: UserInfo[];
}

export function UserList({ users }: Props) {
  return (
    <div className="hidden w-56 flex-col border-l border-border bg-surface sm:flex">
      <div className="flex items-center gap-2 border-b border-border px-3 py-3">
        <Users className="h-4 w-4 text-subtle" />
        <span className="text-xs font-semibold uppercase tracking-wider text-muted">Online</span>
        <Badge tone="accent" className="ml-auto">{users.length}</Badge>
      </div>
      <ul className="flex-1 space-y-0.5 overflow-y-auto p-2 scrollbar-thin">
        {users.map((u) => (
          <li key={u.userId} className="flex items-center gap-2.5 rounded-lg px-2 py-1.5 hover:bg-surface-raised">
            <span
              className="relative flex h-7 w-7 shrink-0 items-center justify-center rounded-full text-[10px] font-semibold text-bg"
              style={{ backgroundColor: userColor(u.userId) }}
              aria-hidden
            >
              {initials(u.username)}
            </span>
            <span className="flex-1 truncate text-sm text-text/90">{u.username}</span>
            <StatusDot tone="success" />
          </li>
        ))}
        {users.length === 0 && (
          <li className="px-2 py-2 text-xs text-subtle">Nobody online</li>
        )}
      </ul>
    </div>
  );
}
