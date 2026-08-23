export function readSetting(key: string): string | null {
  try {
    const raw = localStorage.getItem(key);
    if (raw !== null) return raw;

    const legacy = localStorage.getItem(key.replace(/^okdock\./, 'gamedock.'));
    if (legacy === null) return null;
    localStorage.setItem(key, legacy);
    localStorage.removeItem(key.replace(/^okdock\./, 'gamedock.'));
    return legacy;
  } catch {
    return null;
  }
}
