import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';

const ICONS = new Set(['minecraft', 'terraria', 'valheim', 'palworld', 'ark', 'factorio', 'satisfactory']);

const CATEGORY_COLORS: Record<string, { bg: string; fg: string }> = {
  games: { bg: '#1c3323', fg: '#4fd99b' },
  media: { bg: '#2b2438', fg: '#b79cf0' },
  database: { bg: '#332a1c', fg: '#e5b567' },
  network: { bg: '#1e2f43', fg: '#6aa6f5' },
  utilities: { bg: '#1e222b', fg: '#9297a3' },
  other: { bg: '#1e222b', fg: '#9297a3' },
};

const FAMILY_COLORS: Record<string, { bg: string; fg: string }> = {
  minecraft: { bg: '#1c3323', fg: '#4fd99b' },
  terraria: { bg: '#33261c', fg: '#e5b567' },
  palworld: { bg: '#1e2f43', fg: '#6aa6f5' },
  valheim: { bg: '#2b2438', fg: '#b79cf0' },
  ark: { bg: '#33261c', fg: '#e08a5a' },
  factorio: { bg: '#332a1c', fg: '#e5b567' },
  satisfactory: { bg: '#1e2f43', fg: '#7fb2f0' },
  custom: { bg: '#1e222b', fg: '#9297a3' },
};

export function templateFamily(templateId: string): string {
  return (templateId || '').split('-')[0];
}

export function templateColors(templateId: string, category = 'other'): { bg: string; fg: string } {
  return (
    FAMILY_COLORS[templateFamily(templateId)] ??
    CATEGORY_COLORS[category] ??
    CATEGORY_COLORS['other']
  );
}

@Component({
  selector: 'ok-template-icon',
  changeDetection: ChangeDetectionStrategy.OnPush,
  styles: `
    :host { display: grid; place-items: center; line-height: 0; }
    svg { width: 68%; height: 68%; }
    .letters { font: 600 var(--fs-2xs) var(--mono); line-height: 1; letter-spacing: .02em; }
  `,
  template: `
    @switch (family()) {
      @case ('minecraft') {
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7"
             stroke-linejoin="round" aria-hidden="true">
          <path d="M12 2.6 21 7.6 12 12.6 3 7.6Z" />
          <path d="M3 7.6v8.8l9 5V12.6" />
          <path d="M21 7.6v8.8l-9 5" />
        </svg>
      }
      @case ('terraria') {
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7"
             stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
          <path d="M12 2.4c2.1 0 3.7 1.1 4.4 2.6 2.2.2 3.6 1.9 3.6 3.8 0 2.4-2 4.1-4.6 4.1H8.6C6 12.9 4 11.2 4 8.8c0-1.9 1.4-3.6 3.6-3.8C8.3 3.5 9.9 2.4 12 2.4Z" />
          <path d="M12 12.9V21" />
          <path d="m12 16.6 3.1-3.1" />
          <path d="m12 18.7-2.8-2.8" />
          <path d="M8.6 21h6.8" />
        </svg>
      }
      @case ('valheim') {
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7"
             stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
          <path d="M5 17v-3a7 7 0 0 1 14 0v3" />
          <path d="M5 17h14" />
          <path d="M12 10v7" />
          <path d="M5.5 12C3.5 11 2.5 9 3 6.5c2.2.3 3.8 1.5 4.7 3.4" />
          <path d="M18.5 12c2-1 3-3 2.5-5.5-2.2.3-3.8 1.5-4.7 3.4" />
        </svg>
      }
      @case ('palworld') {
        <svg viewBox="0 0 24 24" fill="currentColor" stroke="none" aria-hidden="true">
          <ellipse cx="7" cy="8.5" rx="2.1" ry="2.7" />
          <ellipse cx="12" cy="6.6" rx="2.1" ry="2.8" />
          <ellipse cx="17" cy="8.5" rx="2.1" ry="2.7" />
          <path d="M12 11.4c3.1 0 5.6 2.3 5.6 5 0 2-1.7 3.2-3.6 2.7-1.3-.4-2.7-.4-4 0-1.9.5-3.6-.7-3.6-2.7 0-2.7 2.5-5 5.6-5Z" />
        </svg>
      }
      @case ('ark') {
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7"
             stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
          <path d="M12 21c-1.6-2-2.4-4-2.4-6" />
          <path d="M12 15c-2.2-1.4-3.6-3.6-4.6-6.6" />
          <path d="M12 15c2.2-1.4 3.6-3.6 4.6-6.6" />
          <path d="M12 15V4.5" />
        </svg>
      }
      @case ('factorio') {
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7"
             stroke-linecap="round" aria-hidden="true">
          <circle cx="12" cy="12" r="4.2" />
          <path d="M12 2.4v3M12 18.6v3M2.4 12h3M18.6 12h3M5.2 5.2 7.3 7.3M16.7 16.7l2.1 2.1M18.8 5.2 16.7 7.3M7.3 16.7l-2.1 2.1" />
        </svg>
      }
      @case ('satisfactory') {
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7"
             stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
          <path d="M3 20V11l5 3V11l5 3V11l5 3v6Z" />
          <path d="M18 8V4h2.5v10" />
          <path d="M3 20h18" />
        </svg>
      }
      @default {
        <span class="letters">{{ fallback() }}</span>
      }
    }
  `,
})
export class TemplateIcon {
  readonly template = input.required<string>();
  readonly fallback = input('··');

  readonly family = computed(() => {
    const f = templateFamily(this.template());
    return ICONS.has(f) ? f : '';
  });
}
