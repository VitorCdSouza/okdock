// the tag list comes from the Hub API, and a first element that looks like a host is not it
export function isHubRepo(repo: string): boolean {
  const parts = repo.trim().split('/');
  if (parts.length === 1) return parts[0] !== '';
  if (parts.length > 2) return false;
  return parts[0] !== 'localhost' && !/[.:]/.test(parts[0]);
}

export function splitImage(ref: string): { repo: string; tag: string } {
  const slash = ref.lastIndexOf('/');
  const colon = ref.lastIndexOf(':');
  if (colon > slash) return { repo: ref.slice(0, colon), tag: ref.slice(colon + 1) };
  return { repo: ref, tag: '' };
}
