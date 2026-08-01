import { AlertTriangle, Play, Radio, Volume2, VolumeX } from 'lucide-react';
import type { AnomalyKind, Briefing, Severity } from '../types/message';
import { cn } from '../lib/cn';
import { Badge } from './ui/Badge';
import { Button } from './ui/Button';
import { Card } from './ui/Card';

interface Props {
  briefings: Briefing[];
  voiceEnabled: boolean;
  needsGesture: boolean;
  onToggleVoice: () => void;
  onPlay: (briefing: Briefing) => void;
}

const kindLabels: Record<AnomalyKind, string> = {
  rate_spike: 'Rate Spike',
  latency_breach: 'Latency Breach',
  connection_surge: 'Connection Surge',
  traffic_dropout: 'Traffic Dropout',
};

const severityTones: Record<Severity, 'warning' | 'danger'> = {
  yellow: 'warning',
  red: 'danger',
};

function formatTime(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return '';
  return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' });
}

function BriefingRow({ briefing, onPlay }: { briefing: Briefing; onPlay: (b: Briefing) => void }) {
  const hasAudio = briefing.audioUrl !== '';

  return (
    <li className="flex items-start gap-3 border-b border-border-subtle py-3 last:border-0">
      <Button
        variant="subtle"
        size="sm"
        className="mt-0.5 shrink-0 px-2"
        onClick={() => onPlay(briefing)}
        disabled={!hasAudio}
        aria-label={hasAudio ? `Play ${kindLabels[briefing.kind] ?? 'briefing'}` : 'Text-only briefing'}
        title={hasAudio ? 'Play briefing' : 'Text-only — audio unavailable'}
      >
        <Play className="h-3.5 w-3.5" />
      </Button>

      <div className="min-w-0 flex-1">
        <div className="mb-1 flex flex-wrap items-center gap-2">
          <Badge tone={severityTones[briefing.severity] ?? 'neutral'}>
            {kindLabels[briefing.kind] ?? briefing.kind}
          </Badge>
          <span className="text-xs text-subtle">{formatTime(briefing.createdAt)}</span>
          {!hasAudio && (
            <span className="text-xs text-subtle" title="TTS unavailable or over budget">
              text-only
            </span>
          )}
        </div>
        {/* The transcript is not decoration: it keeps the panel readable with
            sound off, which is how most demos and most accessibility needs go. */}
        <p className="text-sm leading-relaxed text-muted">{briefing.script}</p>
      </div>
    </li>
  );
}

export function BriefingPanel({ briefings, voiceEnabled, needsGesture, onToggleVoice, onPlay }: Props) {
  return (
    <Card>
      <div className="mb-3 flex items-center justify-between gap-3">
        <div className="flex items-center gap-2 text-subtle">
          <Radio className="h-4 w-4" />
          <p className="text-xs font-medium uppercase tracking-wider">Voice Ops Briefings</p>
        </div>

        <Button
          variant={voiceEnabled ? 'primary' : 'subtle'}
          size="sm"
          onClick={onToggleVoice}
          aria-pressed={voiceEnabled}
        >
          {voiceEnabled ? <Volume2 className="h-3.5 w-3.5" /> : <VolumeX className="h-3.5 w-3.5" />}
          {voiceEnabled ? 'Voice on' : 'Voice off'}
        </Button>
      </div>

      {/* A rejected play() is silent in the console, so it has to be visible here. */}
      {needsGesture && (
        <div
          className={cn(
            'mb-3 flex items-center gap-2 rounded-lg px-3 py-2',
            'bg-warning/10 text-xs text-warning',
          )}
        >
          <AlertTriangle className="h-3.5 w-3.5 shrink-0" />
          <span>Your browser blocked audio. Click a play button to enable it.</span>
        </div>
      )}

      {briefings.length === 0 ? (
        <p className="py-6 text-center text-sm text-subtle">
          No briefings yet. Anomalies in traffic will be narrated here.
        </p>
      ) : (
        <ul className="max-h-80 overflow-y-auto scrollbar-thin">
          {briefings.map((b) => (
            <BriefingRow key={`${b.hash}-${b.createdAt}`} briefing={b} onPlay={onPlay} />
          ))}
        </ul>
      )}
    </Card>
  );
}
