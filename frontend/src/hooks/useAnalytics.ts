import { useEffect, useState } from 'react';
import type { AnalyticsMetrics } from '../types/message';

const API_URL = import.meta.env.VITE_API_URL ?? 'http://localhost:8080';

interface UseAnalyticsOptions {
  /** When false, polling pauses (e.g. analytics tab is hidden). Defaults to true. */
  enabled?: boolean;
}

export function useAnalytics(intervalMs = 5000, options: UseAnalyticsOptions = {}) {
  const { enabled = true } = options;
  const [metrics, setMetrics] = useState<AnalyticsMetrics | null>(null);

  useEffect(() => {
    if (!enabled) return;

    let cancelled = false;
    const controller = new AbortController();

    const fetch_ = async () => {
      try {
        const res = await fetch(`${API_URL}/api/analytics`, { signal: controller.signal });
        if (!res.ok) return;
        const data = (await res.json()) as AnalyticsMetrics;
        if (!cancelled) setMetrics(data);
      } catch {
        // network error / abort — keep last value
      }
    };

    fetch_();
    const id = setInterval(fetch_, intervalMs);
    return () => {
      cancelled = true;
      controller.abort();
      clearInterval(id);
    };
  }, [intervalMs, enabled]);

  return { metrics };
}
