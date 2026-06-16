import { type KeyboardEvent, useState } from 'react';
import { Send } from 'lucide-react';
import { Badge } from './ui/Badge';
import { Button } from './ui/Button';

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
    <div className="border-t border-border bg-surface p-3">
      <div className="flex items-end gap-2">
        <textarea
          className="flex-1 resize-none rounded-lg border border-border bg-surface-raised p-2.5 text-sm text-text placeholder:text-subtle focus:border-accent focus:outline-none focus:ring-2 focus:ring-accent/40 disabled:opacity-50"
          rows={2}
          maxLength={MAX_CHARS}
          placeholder={disabled ? 'Connecting…' : 'Type a message (Enter to send, Shift+Enter for newline)'}
          value={value}
          disabled={disabled}
          onChange={(e) => setValue(e.target.value)}
          onKeyDown={handleKeyDown}
        />
        <div className="flex flex-col items-end gap-1.5">
          <Badge tone={remaining < 50 ? 'danger' : 'neutral'}>{remaining}</Badge>
          <Button onClick={submit} disabled={disabled || !value.trim()}>
            <Send className="h-4 w-4" />
            <span className="hidden sm:inline">Send</span>
          </Button>
        </div>
      </div>
    </div>
  );
}
