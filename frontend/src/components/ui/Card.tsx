import type { HTMLAttributes } from 'react';
import { cn } from '../../lib/cn';

interface Props extends HTMLAttributes<HTMLDivElement> {
  padding?: 'none' | 'sm' | 'md' | 'lg';
}

const paddings = {
  none: '',
  sm: 'p-3',
  md: 'p-4',
  lg: 'p-6',
};

export function Card({ padding = 'md', className, ...props }: Props) {
  return (
    <div
      className={cn(
        'rounded-xl border border-border-subtle bg-surface-raised',
        paddings[padding],
        className,
      )}
      {...props}
    />
  );
}
