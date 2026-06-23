/**
 * Small formatting helpers used in tables and cards. No bespoke styling here —
 * just value formatting.
 */

const UNITS = ["B", "KB", "MB", "GB", "TB", "PB"] as const;

/** Format a byte count into a human-readable string (B/KB/MB/GB/TB/PB). */
export function formatBytes(bytes: number | null | undefined, decimals = 1): string {
  if (bytes === null || bytes === undefined || Number.isNaN(bytes)) return "—";
  if (bytes <= 0) return "0 B";
  const i = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), UNITS.length - 1);
  const value = bytes / Math.pow(1024, i);
  // No decimals for plain bytes.
  const fixed = i === 0 ? 0 : decimals;
  return `${value.toFixed(fixed)} ${UNITS[i]}`;
}

/** Format a count with thousands separators. */
export function formatNumber(n: number | null | undefined): string {
  if (n === null || n === undefined || Number.isNaN(n)) return "—";
  return new Intl.NumberFormat().format(n);
}

/** Format an ISO date string as a readable local date-time. */
export function formatDate(input: string | number | Date | null | undefined): string {
  if (!input) return "—";
  const d = input instanceof Date ? input : new Date(input);
  if (Number.isNaN(d.getTime())) return "—";
  return d.toLocaleString(undefined, {
    year: "numeric",
    month: "short",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  });
}

/** Format an ISO date string as just the date portion. */
export function formatDateShort(input: string | number | Date | null | undefined): string {
  if (!input) return "—";
  const d = input instanceof Date ? input : new Date(input);
  if (Number.isNaN(d.getTime())) return "—";
  return d.toLocaleDateString(undefined, {
    year: "numeric",
    month: "short",
    day: "2-digit",
  });
}
