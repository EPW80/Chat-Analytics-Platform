import { forwardRef, type InputHTMLAttributes } from 'react';
import { cn } from '../../lib/cn';

export const Input = forwardRef<HTMLInputElement, InputHTMLAttributes<HTMLInputElement>>(
  function Input({ className, ...props }, ref) {
    return (
      <input
        ref={ref}
        className={cn(
          'w-full rounded-lg border border-border bg-surface px-3 py-2 text-sm text-text',
          'placeholder:text-subtle',
          'focus:border-accent focus:outline-none focus:ring-2 focus:ring-accent/40',
          'disabled:opacity-50',
          className,
        )}
        {...props}
      />
    );
  },
);
