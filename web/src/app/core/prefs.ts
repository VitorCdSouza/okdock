import { Injectable, effect, signal } from '@angular/core';

export interface MetricPrefs {
  cpu: boolean;
  memory: boolean;
  disk: boolean;
  budget: boolean;
}

const DEFAULTS: MetricPrefs = { cpu: true, memory: true, disk: true, budget: true };
const KEY = 'gamedock.metrics';

@Injectable({ providedIn: 'root' })
export class Prefs {
  readonly metrics = signal<MetricPrefs>(load());

  constructor() {
    effect(() => {
      try {
        localStorage.setItem(KEY, JSON.stringify(this.metrics()));
      } catch {
      }
    });
  }

  toggle(key: keyof MetricPrefs): void {
    this.metrics.update((m) => ({ ...m, [key]: !m[key] }));
  }
}

function load(): MetricPrefs {
  try {
    const raw = localStorage.getItem(KEY);
    if (!raw) return { ...DEFAULTS };
    return { ...DEFAULTS, ...(JSON.parse(raw) as Partial<MetricPrefs>) };
  } catch {
    return { ...DEFAULTS };
  }
}
