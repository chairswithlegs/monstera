import { cn } from '@/lib/utils';

type EmptyStateProps = {
  message: string;
  className?: string;
};

export function EmptyState({ message, className }: EmptyStateProps) {
  return (
    <p className={cn('p-6 text-muted-foreground', className)}>
      {message}
    </p>
  );
}
