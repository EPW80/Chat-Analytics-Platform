import { type KeyboardEvent, useState } from 'react';

interface Props {
  onSend: (content: string) => void;
  disabled: boolean;
}

const MAX_CHARS = 500;

export function MessageInput({ onSend, disabled }: Props) {
  const [value, setValue] = useState('');

  const submit = () => {
    const trimmed = value.trim();
    if (!trimmed || disabled) return;
    onSend(trimmed);
    setValue('');
  };

  const handleKeyDown = (e: KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      submit();
    }
  };

  const remaining = MAX_CHARS - value.length;

  return (
    <div className="border-t border-gray-700 bg-gray-900 p-3">
      <div className="flex gap-2 items-end">
        <textarea
          className="flex-1 resize-none rounded bg-gray-800 text-white placeholder-gray-500 p-2 text-sm focus:outline-none focus:ring-1 focus:ring-indigo-500 disabled:opacity-50"
          rows={2}
          maxLength={MAX_CHARS}
          placeholder={disabled ? 'Connecting…' : 'Type a message (Enter to send, Shift+Enter for newline)'}
          value={value}
          disabled={disabled}
          onChange={(e) => setValue(e.target.value)}
          onKeyDown={handleKeyDown}
        />
        <div className="flex flex-col items-end gap-1">
          <span className={`text-xs ${remaining < 50 ? 'text-red-400' : 'text-gray-500'}`}>
            {remaining}
          </span>
          <button
            onClick={submit}
            disabled={disabled || !value.trim()}
            className="px-4 py-1.5 rounded bg-indigo-600 text-white text-sm font-medium hover:bg-indigo-500 disabled:opacity-40 disabled:cursor-not-allowed"
          >
            Send
          </button>
        </div>
      </div>
    </div>
  );
}
