import { describe, it, expect } from 'vitest';
import { formatBytes, formatBytesPerSecond } from '../bytes';

describe('formatBytes', () => {
  it('returns 0 B for 0 or negative values', () => {
    expect(formatBytes(0)).toBe('0 B');
    expect(formatBytes(-1)).toBe('0 B');
  });

  it('formats bytes to KB, MB, GB, TB correctly', () => {
    expect(formatBytes(1)).toBe('1.00 B');
    expect(formatBytes(1024)).toBe('1.00 KB');
    expect(formatBytes(1024 * 1024)).toBe('1.00 MB');
    expect(formatBytes(1024 * 1024 * 1024)).toBe('1.00 GB');
  });

  it('formats numbers with dynamic decimal places', () => {
    // Values < 10: 2 decimals
    const twoPointFiveGB = 2.5 * 1024 * 1024 * 1024;
    expect(formatBytes(twoPointFiveGB)).toBe('2.50 GB');

    // Values >= 10 and < 100: 1 decimal
    const tenPointFiveGB = 10.5 * 1024 * 1024 * 1024;
    expect(formatBytes(tenPointFiveGB)).toBe('10.5 GB');

    // Values >= 100: 0 decimals
    const hundredGB = 100 * 1024 * 1024 * 1024;
    expect(formatBytes(hundredGB)).toBe('100 GB');
  });

  it('formats petabyte and 64-bit byte totals with defined units', () => {
    expect(formatBytes(1024 ** 5)).toBe('1.00 PB');
    expect(formatBytes(Number(2n ** 63n - 1n))).toBe('8.00 EB');
    expect(formatBytes(1024 ** 7)).toBe('1024 EB');
    expect(formatBytesPerSecond(1024 ** 5)).toBe('1.00 PB/s');
  });

  it('handles non-finite and sub-byte input without an undefined unit', () => {
    expect(formatBytes(Infinity)).toBe('0 B');
    expect(formatBytes(NaN)).toBe('0 B');
    expect(formatBytes(0.5)).toBe('0.50 B');
  });
});


