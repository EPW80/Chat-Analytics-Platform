import { describe, expect, it } from 'vitest';
import { initials, userColor } from './userColor';

describe('userColor', () => {
  it('returns a stable color for the same user id', () => {
    expect(userColor('alice')).toBe(userColor('alice'));
  });

  it('returns different colors for different ids when hashes diverge', () => {
    // Not guaranteed for every pair, but these two must differ with the current palette.
    expect(userColor('alice')).not.toBe(userColor('bob'));
  });
});

describe('initials', () => {
  it('returns ? for empty names', () => {
    expect(initials('')).toBe('?');
    expect(initials('   ')).toBe('?');
  });

  it('uses up to two characters for a single name', () => {
    expect(initials('Ada')).toBe('AD');
  });

  it('uses first and last name initials', () => {
    expect(initials('Ada Lovelace')).toBe('AL');
  });
});
