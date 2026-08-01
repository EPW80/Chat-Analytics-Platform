import { useCallback, useEffect, useRef, useState } from 'react';
import type { Briefing } from '../types/message';

const VOICE_KEY = 'briefings.voiceEnabled';
const MAX_BRIEFINGS = 50;

const API_URL = import.meta.env.VITE_API_URL ?? 'http://localhost:8080';

interface UseBriefingsReturn {
  briefings: Briefing[];
  voiceEnabled: boolean;
  toggleVoice: () => void;
  /** True when the browser refused playback and needs a fresh user gesture. */
  needsGesture: boolean;
  /** Call from the WebSocket's `briefing.ready` branch. */
  onBriefingFrame: (briefing: Briefing) => void;
  /** Play one briefing on demand — this call is itself a user gesture. */
  play: (briefing: Briefing) => void;
}

function readStoredPreference(): boolean {
  try {
    return localStorage.getItem(VOICE_KEY) === 'true';
  } catch {
    // Private-mode browsers throw on localStorage access. Default to silence.
    return false;
  }
}

export function useBriefings(): UseBriefingsReturn {
  const [briefings, setBriefings] = useState<Briefing[]>([]);
  // Default OFF. The stored value is the user's persisted choice.
  const [voiceEnabled, setVoiceEnabled] = useState<boolean>(readStoredPreference);
  const [needsGesture, setNeedsGesture] = useState(false);

  // Read through a ref: onBriefingFrame is handed to useWebSocket once, and a
  // dependency on voiceEnabled would leave it holding a stale value.
  const voiceEnabledRef = useRef(voiceEnabled);
  voiceEnabledRef.current = voiceEnabled;

  const audioRef = useRef<HTMLAudioElement | null>(null);

  // Seed from the REST endpoint so a page load shows history, not an empty panel.
  useEffect(() => {
    let cancelled = false;
    fetch(`${API_URL}/api/briefings?limit=${MAX_BRIEFINGS}`)
      .then((res) => (res.ok ? res.json() : null))
      .then((data: { briefings?: Briefing[] } | null) => {
        if (cancelled || !data?.briefings) return;
        setBriefings(data.briefings);
      })
      .catch(() => {
        // Briefings may be disabled server-side (404). An empty panel is correct.
      });
    return () => {
      cancelled = true;
    };
  }, []);

  const play = useCallback((briefing: Briefing) => {
    if (!briefing.audioUrl) return;

    // One element, reused: a burst of briefings should replace the current
    // narration rather than talk over it.
    audioRef.current?.pause();
    const audio = new Audio(`${API_URL}${briefing.audioUrl}`);
    audioRef.current = audio;

    audio
      .play()
      .then(() => setNeedsGesture(false))
      .catch(() => {
        // IMPORTANT: browsers reject play() before a user gesture *silently* —
        // no console error. Surface it instead of swallowing it.
        setNeedsGesture(true);
      });
  }, []);

  const toggleVoice = useCallback(() => {
    setVoiceEnabled((prev) => {
      const next = !prev;
      try {
        localStorage.setItem(VOICE_KEY, String(next));
      } catch {
        // Preference just won't persist; playback still works this session.
      }
      // The click that got us here is the gesture that unlocks audio.
      if (next) setNeedsGesture(false);
      return next;
    });
  }, []);

  const onBriefingFrame = useCallback(
    (briefing: Briefing) => {
      setBriefings((prev) => [briefing, ...prev].slice(0, MAX_BRIEFINGS));
      if (voiceEnabledRef.current) play(briefing);
    },
    [play]
  );

  // Stop narration if the component goes away mid-briefing.
  useEffect(() => {
    return () => audioRef.current?.pause();
  }, []);

  return { briefings, voiceEnabled, toggleVoice, needsGesture, onBriefingFrame, play };
}
