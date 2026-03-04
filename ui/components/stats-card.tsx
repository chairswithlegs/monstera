import * as React from 'react';
import { Card, CardContent } from '@/components/ui/card';
import { cn } from '@/lib/utils';

type StatsCardProps = {
  label: string;
  value: React.ReactNode;
  className?: string;
};

export function StatsCard({ label, value, className }: StatsCardProps) {
  return (
    <Card className={cn('py-4', className)}>
      <CardContent className="px-4 pt-0">
        <dl>
          <dt className="text-sm font-medium text-muted-foreground">{label}</dt>
          <dd className="mt-1 text-2xl font-semibold text-foreground">{value}</dd>
        </dl>
      </CardContent>
    </Card>
  );
}
