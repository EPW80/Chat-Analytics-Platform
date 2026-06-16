import {
  Activity,
  BarChart3,
  Clock,
  Gauge,
  MessageSquare,
  TrendingUp,
  Users,
  type LucideIcon,
} from 'lucide-react';
import type { AnalyticsMetrics } from '../types/message';
import { Card } from './ui/Card';

interface Props {
  metrics: AnalyticsMetrics | null;
}

function MetricCard({
  label,
  value,
  unit = '',
  Icon,
}: {
  label: string;
  value: string | number;
  unit?: string;
  Icon: LucideIcon;
}) {
  return (
    <Card className="flex flex-col gap-2">
      <div className="flex items-center gap-2 text-subtle">
        <Icon className="h-4 w-4" />
        <span className="text-xs font-medium uppercase tracking-wider">{label}</span>
      </div>
      <span className="text-2xl font-bold text-text">
        {value}
        {unit && <span className="ml-1 text-sm font-medium text-muted">{unit}</span>}
      </span>
    </Card>
  );
}

function MiniBar({ count, max }: { count: number; max: number }) {
  const pct = max > 0 ? Math.round((count / max) * 100) : 0;
  return (
    <div className="group flex flex-1 flex-col items-center gap-1" title={`${count} messages`}>
      <div className="flex w-full items-end" style={{ height: 56 }}>
        <div
          className="w-full rounded-t bg-accent transition-colors group-hover:bg-accent-hover"
          style={{ height: `${pct}%`, minHeight: count > 0 ? 2 : 0 }}
        />
      </div>
      <span className="text-[10px] text-subtle">{count}</span>
    </div>
  );
}

function formatUptime(seconds: number): string {
  const h = Math.floor(seconds / 3600);
  const m = Math.floor((seconds % 3600) / 60);
  const s = seconds % 60;
  if (h > 0) return `${h}h ${m}m`;
  if (m > 0) return `${m}m ${s}s`;
  return `${s}s`;
}

export function AnalyticsDashboard({ metrics }: Props) {
  if (!metrics) {
    return (
      <div className="flex flex-1 flex-col items-center justify-center gap-3 text-subtle">
        <Gauge className="h-10 w-10 animate-pulse opacity-40" />
        <p className="text-sm">Loading metrics…</p>
      </div>
    );
  }

  const mpm = metrics.messagesPerMinute ?? [];
  const maxMpm = Math.max(...mpm, 1);

  return (
    <div className="flex-1 space-y-6 overflow-y-auto p-6 scrollbar-thin">
      <div className="grid grid-cols-2 gap-4 md:grid-cols-4">
        <MetricCard label="Total Messages" value={metrics.totalMessages} Icon={MessageSquare} />
        <MetricCard label="Active Users" value={metrics.activeUsers} Icon={Users} />
        <MetricCard label="Peak Connections" value={metrics.peakConnections} Icon={TrendingUp} />
        <MetricCard label="Latency P95" value={metrics.latencyP95Ms.toFixed(2)} unit="ms" Icon={Gauge} />
      </div>

      <Card>
        <div className="mb-4 flex items-center gap-2 text-subtle">
          <BarChart3 className="h-4 w-4" />
          <p className="text-xs font-medium uppercase tracking-wider">Messages / Minute — last 15 min</p>
        </div>
        <div className="flex items-end gap-1">
          {mpm.map((count, i) => (
            <MiniBar key={i} count={count} max={maxMpm} />
          ))}
        </div>
      </Card>

      <div className="grid grid-cols-2 gap-4 md:grid-cols-3">
        <MetricCard label="Latency P50" value={metrics.latencyP50Ms.toFixed(2)} unit="ms" Icon={Activity} />
        <MetricCard label="Latency P99" value={metrics.latencyP99Ms.toFixed(2)} unit="ms" Icon={Gauge} />
        <MetricCard label="Server Uptime" value={formatUptime(metrics.uptimeSeconds)} Icon={Clock} />
      </div>
    </div>
  );
}
