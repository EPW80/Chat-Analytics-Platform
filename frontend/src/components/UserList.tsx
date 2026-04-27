import type { UserInfo } from '../types/message';

interface Props {
  users: UserInfo[];
}

export function UserList({ users }: Props) {
  return (
    <div className="w-44 border-l border-gray-700 bg-gray-900 flex flex-col">
      <div className="px-3 py-2 border-b border-gray-700">
        <span className="text-xs font-semibold text-gray-400 uppercase tracking-wider">
          Online — {users.length}
        </span>
      </div>
      <ul className="flex-1 overflow-y-auto py-2">
        {users.map((u) => (
          <li key={u.userId} className="flex items-center gap-2 px-3 py-1">
            <span className="w-2 h-2 rounded-full bg-green-400 flex-shrink-0" />
            <span className="text-sm text-gray-300 truncate">{u.username}</span>
          </li>
        ))}
        {users.length === 0 && (
          <li className="px-3 py-1 text-xs text-gray-600">Nobody online</li>
        )}
      </ul>
    </div>
  );
}
