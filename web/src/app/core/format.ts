export function bytes(n: number | undefined | null): string {
  if (!n || n < 0) return '0 B';
  const gb = 1024 ** 3;
  const mb = 1024 ** 2;
  if (n >= gb) return `${(n / gb).toFixed(n / gb >= 10 ? 0 : 1)} GB`;
  if (n >= mb) return `${Math.round(n / mb)} MB`;
  if (n >= 1024) return `${Math.round(n / 1024)} KB`;
  return `${n} B`;
}

export function gigabytes(n: number | undefined | null): string {
  if (!n || n < 0) return '0';
  const gb = n / 1024 ** 3;
  return gb >= 10 ? gb.toFixed(0) : gb.toFixed(1);
}

export function pct(value: number, total: number): number {
  if (!total) return 0;
  return Math.min(100, Math.max(0, (value / total) * 100));
}
