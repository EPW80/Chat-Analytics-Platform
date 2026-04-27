import type { AnalyticsMetrics } from '../types/message';

interface Props {
  metrics: AnalyticsMetrics | null;
}

function MetricCard({ label, value, unit = '' }: { label: string; value: string | number; unit?: string }) {
  return (
    <div className="bg-gray-800 rounded-lg p-4 flex flex-col gap-1">
      <span className="text-xs text-gray-400 uppercase tracking-wider">{label}</span>
      <span className="text-2xl font-bold text-white">
        {value}<span className="text-sm text-gray-400 ml-1">{unit}</span>
      </span>
    </div>
  );
}

function MiniBar({ count, max }: { count: number; max: number }) {
  const pct = max > 0 ? Math.round((count / max) * 100) : 0;
  return (
    <div className="flex flex-col items-center gap-0.5 flex-1">
      <div className="w-full flex items-end" style={{ height: 48 }}>
        <div
          className="w-full bg-indigo-500 rounded-t"
          style={{ height: `${pct}%`, minHeight: count > 0 ? 2 : 0 }}
        />
      </div>
      <span className="text-xs text-gray-500">{count}</span>
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
      <div className="flex-1 flex items-center justify-center text-gray-500">
        Loading metrics…
      </div>
    );
  }

  const mpm = metrics.messagesPerMinute ?? [];
  const maxMpm = Math.max(...mpm, 1);

  return (
    <div className="flex-1 p-6 overflow-y-auto space-y-6">
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
        <MetricCard label="Total Messages" value={metrics.totalMessages} />
        <MetricCard label="Active Users" value={metrics.activeUsers} />
        <MetricCard label="Peak Connections" value={metrics.peakConnections} />
        <MetricCard label="Latency P95" value={metrics.latencyP95Ms.toFixed(2)} unit="ms" />
      </div>

      <div className="bg-gray-800 rounded-lg p-4">
        <p className="text-xs text-gray-400 uppercase tracking-wider mb-3">
          Messages / Minute — last 15 min
        </p>
        <div className="flex items-end gap-1">
          {mpm.map((count, i) => (
            <MiniBar key={i} count={count} max={maxMpm} />
          ))}
        </div>
      </div>

      <div className="grid grid-cols-2 md:grid-cols-3 gap-4">
        <MetricCard label="Latency P50" value={metrics.latencyP50Ms.toFixed(2)} unit="ms" />
        <MetricCard label="Latency P99" value={metrics.latencyP99Ms.toFixed(2)} unit="ms" />
        <MetricCard label="Server Uptime" value={formatUptime(metrics.uptimeSeconds)} />
      </div>
    </div>
  );
}
