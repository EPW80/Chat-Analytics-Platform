import type { Briefing, InboundFrame, Message } from '../types/message';

export interface ParsedFrames {
  messages: Message[];
  briefings: Briefing[];
}

/** Parse one or more newline-delimited inbound WebSocket frames. */
export function parseInboundFrames(raw: string): ParsedFrames {
  const lines = raw.split('\n').filter(Boolean);
  const messages: Message[] = [];
  const briefings: Briefing[] = [];

  for (const line of lines) {
    try {
      const frame = JSON.parse(line) as InboundFrame;
      if (frame.type === 'briefing.ready') {
        briefings.push(frame.briefing);
      } else if (
        frame.type === 'chat' ||
        frame.type === 'system' ||
        frame.type === 'join' ||
        frame.type === 'leave'
      ) {
        messages.push(frame);
      }
    } catch {
      // ignore malformed lines
    }
  }

  return { messages, briefings };
}
