export function formatBytes(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes <= 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB', 'PB', 'EB'];
  const i = Math.max(0, Math.min(sizes.length - 1, Math.floor(Math.log(bytes) / Math.log(k))));
  const value = bytes / Math.pow(k, i);
  return `${value.toFixed(value >= 100 ? 0 : value >= 10 ? 1 : 2)} ${sizes[i]}`;
}

export function formatBytesPerSecond(bytesPerSecond: number): string {
  if (!bytesPerSecond || bytesPerSecond <= 0) return '0 B/s';
  return `${formatBytes(bytesPerSecond)}/s`;
}

/**
 * Speed limit segments in bytes per second
 * More granular between 100KB and 50MB, fewer segments above 50MB
 */
export const SPEED_LIMIT_SEGMENTS = [
  100 * 1024,      // 100 KB
  200 * 1024,      // 200 KB
  300 * 1024,      // 300 KB
  400 * 1024,      // 400 KB
  500 * 1024,      // 500 KB
  600 * 1024,      // 600 KB
  700 * 1024,      // 700 KB
  800 * 1024,      // 800 KB
  900 * 1024,      // 900 KB
  1024 * 1024,     // 1 MB
  1.5 * 1024 * 1024, // 1.5 MB
  2 * 1024 * 1024, // 2 MB
  2.5 * 1024 * 1024, // 2.5 MB
  3 * 1024 * 1024, // 3 MB
  4 * 1024 * 1024, // 4 MB
  5 * 1024 * 1024, // 5 MB
  6 * 1024 * 1024, // 6 MB
  7 * 1024 * 1024, // 7 MB
  8 * 1024 * 1024, // 8 MB
  10 * 1024 * 1024, // 10 MB
  12 * 1024 * 1024, // 12 MB
  15 * 1024 * 1024, // 15 MB
  20 * 1024 * 1024, // 20 MB
  25 * 1024 * 1024, // 25 MB
  30 * 1024 * 1024, // 30 MB
  40 * 1024 * 1024, // 40 MB
  50 * 1024 * 1024, // 50 MB
  75 * 1024 * 1024, // 75 MB
  100 * 1024 * 1024, // 100 MB
  150 * 1024 * 1024, // 150 MB
  200 * 1024 * 1024, // 200 MB
  300 * 1024 * 1024, // 300 MB
  500 * 1024 * 1024, // 500 MB
  1024 * 1024 * 1024, // 1 GB
  2 * 1024 * 1024 * 1024, // 2 GB
  5 * 1024 * 1024 * 1024, // 5 GB
  10 * 1024 * 1024 * 1024, // 10 GB
] as const;

/**
 * Convert bytes value to slider index (1-based, where max index = unlimited)
 * Index 1 to SPEED_LIMIT_SEGMENTS.length = segment values
 * Index SPEED_LIMIT_SEGMENTS.length + 1 = unlimited (0)
 */
export function bytesToSliderIndex(bytes: number): number {
  if (bytes === 0) return SPEED_LIMIT_SEGMENTS.length + 1; // Unlimited is max index

  // Find the first segment that is >= bytes
  for (let i = 0; i < SPEED_LIMIT_SEGMENTS.length; i++) {
    if (bytes <= SPEED_LIMIT_SEGMENTS[i]) {
      return i + 1; // +1 because slider starts at 1
    }
  }

  // If bytes is greater than all segments, return unlimited (max index)
  return SPEED_LIMIT_SEGMENTS.length + 1;
}

/**
 * Convert slider index to bytes value
 * Index 1 = first segment, Index max = unlimited (0)
 */
export function sliderIndexToBytes(index: number): number {
  if (index > SPEED_LIMIT_SEGMENTS.length) {
    return 0; // Max index = unlimited (0)
  }
  if (index <= 0) {
    return SPEED_LIMIT_SEGMENTS[0]; // Safety fallback
  }
  return SPEED_LIMIT_SEGMENTS[index - 1];
}
