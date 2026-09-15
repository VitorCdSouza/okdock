import { Injectable, effect, signal } from '@angular/core';

import { readSetting } from './storage';

export interface MetricPrefs {
  cpu: boolean;
  memory: boolean;
  disk: boolean;
  budget: boolean;
}

const DEFAULTS: MetricPrefs = { cpu: true, memory: true, disk: true, budget: true };
const KEY = 'okdock.metrics';
const SELF_KEY = 'okdock.showSelf';

@Injectable({ providedIn: 'root' })
export class Prefs {
  readonly metrics = signal<MetricPrefs>(load());
  // the panel card is off by default, it takes no action and only takes room
  readonly showSelf = signal(readSetting(SELF_KEY) === 'true');

  constructor() {
    effect(() => {
      try {
        localStorage.setItem(KEY, JSON.stringify(this.metrics()));
      } catch {
      }
    });
    effect(() => {
      try {
        localStorage.setItem(SELF_KEY, String(this.showSelf()));
      } catch {
      }
    });
  }

  setMetrics(metrics: MetricPrefs): void {
    this.metrics.set({ ...metrics });
  }
}

function load(): MetricPrefs {
  try {
    const raw = readSetting(KEY);
    if (!raw) return { ...DEFAULTS };
    return { ...DEFAULTS, ...(JSON.parse(raw) as Partial<MetricPrefs>) };
  } catch {
    return { ...DEFAULTS };
  }
}
