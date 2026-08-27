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
