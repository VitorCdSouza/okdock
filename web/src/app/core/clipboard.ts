export function copyText(text: string): void {
  if (!text) return;

  const fallback = () => {
    const area = document.createElement('textarea');
    area.value = text;
    area.setAttribute('readonly', '');
    area.style.position = 'fixed';
    area.style.opacity = '0';
    document.body.appendChild(area);
    area.select();
    document.execCommand('copy');
    document.body.removeChild(area);
  };

  if (navigator.clipboard?.writeText) {
    navigator.clipboard.writeText(text).catch(fallback);
    return;
  }
  fallback();
}
