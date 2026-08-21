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

export function since(iso: string | Date | undefined): string {
  if (!iso) return '';
  const then = typeof iso === 'string' ? new Date(iso) : iso;
  const secs = Math.max(0, (Date.now() - then.getTime()) / 1000);
  if (secs < 60) return 'há segundos';
  const mins = Math.floor(secs / 60);
  if (mins < 60) return `há ${mins} min`;
  const hours = Math.floor(mins / 60);
  if (hours < 24) return `há ${hours}h`;
  const days = Math.floor(hours / 24);
  if (days < 30) return `há ${days}d`;
  const months = Math.floor(days / 30);
  return months < 12 ? `há ${months} meses` : `há ${Math.floor(months / 12)} anos`;
}

export function pct(value: number, total: number): number {
  if (!total) return 0;
  return Math.min(100, Math.max(0, (value / total) * 100));
}
