import { describe, expect, it } from 'vitest';
import { parseInboundFrames } from './wsFrames';

describe('parseInboundFrames', () => {
  it('parses chat frames and ignores junk', () => {
    const raw = [
      JSON.stringify({
        type: 'chat',
        messageId: '1',
        roomId: 'global',
        userId: 'u1',
        username: 'Ada',
        content: 'hi',
        timestamp: '2026-01-01T00:00:00Z',
      }),
      'not-json',
      JSON.stringify({ type: 'unknown' }),
    ].join('\n');

    const { messages, briefings } = parseInboundFrames(raw);
    expect(messages).toHaveLength(1);
    expect(messages[0]?.content).toBe('hi');
    expect(briefings).toHaveLength(0);
  });

  it('routes briefing.ready frames separately', () => {
    const briefing = {
      hash: 'abc',
      script: 'spike detected',
      audioUrl: '/api/briefings/abc/audio',
      createdAt: '2026-01-01T00:00:00Z',
    };
    const raw = JSON.stringify({ type: 'briefing.ready', briefing });
    const { messages, briefings } = parseInboundFrames(raw);
    expect(messages).toHaveLength(0);
    expect(briefings).toEqual([briefing]);
  });
});
