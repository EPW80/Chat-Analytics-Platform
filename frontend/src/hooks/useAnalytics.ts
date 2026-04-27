import { useEffect, useState } from 'react';
import type { AnalyticsMetrics } from '../types/message';

const API_URL = import.meta.env.VITE_API_URL ?? 'http://localhost:8080';

export function useAnalytics(intervalMs = 5000) {
  const [metrics, setMetrics] = useState<AnalyticsMetrics | null>(null);

  useEffect(() => {
    let cancelled = false;

    const fetch_ = async () => {
      try {
        const res = await fetch(`${API_URL}/api/analytics`);
        if (!res.ok) return;
        const data = (await res.json()) as AnalyticsMetrics;
        if (!cancelled) setMetrics(data);
      } catch {
        // network error — keep last value
      }
    };

    fetch_();
    const id = setInterval(fetch_, intervalMs);
    return () => {
      cancelled = true;
      clearInterval(id);
    };
  }, [intervalMs]);

  return { metrics };
}
