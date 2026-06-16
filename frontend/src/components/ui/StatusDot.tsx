import { cn } from '../../lib/cn';

type Tone = 'success' | 'warning' | 'danger' | 'neutral';

interface Props {
  tone?: Tone;
  pulse?: boolean;
  className?: string;
}

const tones: Record<Tone, string> = {
  success: 'bg-success',
  warning: 'bg-warning',
  danger: 'bg-danger',
  neutral: 'bg-subtle',
};

export function StatusDot({ tone = 'neutral', pulse = false, className }: Props) {
  return (
    <span
      className={cn(
        'inline-block h-2 w-2 shrink-0 rounded-full',
        tones[tone],
        pulse && 'animate-pulse',
        className,
      )}
    />
  );
}
