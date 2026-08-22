import { Injectable, computed, inject, signal } from '@angular/core';

import { Api } from './api';
import { Events } from './events';
import { I18n } from './i18n/i18n';
import { DnsStatus, Instance, Provider, State, SystemInfo } from './models';

@Injectable({ providedIn: 'root' })
export class Store {
  private readonly api = inject(Api);
  private readonly events = inject(Events);
  private readonly i18n = inject(I18n);

  readonly instances = signal<Instance[]>([]);
  readonly states = signal<State[]>([]);
  readonly providers = signal<Provider[]>([]);
  readonly system = signal<SystemInfo | null>(null);
  readonly dns = signal<DnsStatus | null>(null);
  readonly loading = signal(true);
  readonly gameFilter = signal<string | null>(null);
  readonly search = signal('');

  readonly dragging = signal<string | null>(null);
  readonly toast = signal<string | null>(null);

  private reloadTimer?: ReturnType<typeof setTimeout>;
  private toastTimer?: ReturnType<typeof setTimeout>;
  private started = false;

  notify(message: string): void {
    this.toast.set(message);
    clearTimeout(this.toastTimer);
    this.toastTimer = setTimeout(() => this.toast.set(null), 6000);
  }

  readonly filtered = computed(() => {
    const term = this.search().trim().toLowerCase();
    const game = this.gameFilter();
    return this.instances().filter((i) => {
      if (game && i.game !== game) return false;
      if (!term) return true;
      const haystack = [
        i.name,
        i.image,
        i.providerId,
        ...i.ports.map((p) => String(p.host)),
      ]
        .join(' ')
        .toLowerCase();
      return haystack.includes(term);
    });
  });

  readonly gameCounts = computed(() => {
    const counts = new Map<string, { label: string; count: number }>();
    for (const i of this.instances()) {
      const label = this.providers().find((p) => p.game === i.game)?.gameLabel ?? i.game;
      const cur = counts.get(i.game);
      counts.set(i.game, { label, count: (cur?.count ?? 0) + 1 });
    }
    return [...counts.entries()].map(([game, v]) => ({ game, ...v }));
  });

  byState(state: State): Instance[] {
    return this.filtered().filter((i) => i.state === state);
  }

  start(): void {
    if (this.started) return;
    this.started = true;

    this.api.providers().subscribe({
      next: (p) => this.providers.set(p),
      error: () => {},
    });
    this.reload();
    this.reloadDns();

    this.events.stream().subscribe({
      next: (ev) => {
        if (ev.type === 'instance.uptodate' || ev.type === 'instance.updated') {
          const text =
            this.i18n.maybe(`event.${ev.type}`, { name: ev.instance ?? '' }) ?? ev.message;
          if (text) this.notify(text);
        }
        if (ev.type === 'dns.changed') this.reloadDns();
        this.scheduleReload();
      },
      error: () => setInterval(() => this.reload(), 5000),
    });
  }

  reload(): void {
    this.api.instances().subscribe({
      next: ({ instances, states }) => {
        this.instances.set(instances);
        this.states.set(states);
        this.loading.set(false);
      },
      error: () => this.loading.set(false),
    });
    this.api.system().subscribe({
      next: (s) => this.system.set(s),
      error: () => {},
    });
  }

  reloadDns(): void {
    this.api.dns().subscribe({
      next: (d) => this.dns.set(d),
      error: () => {},
    });
  }

  private scheduleReload(): void {
    clearTimeout(this.reloadTimer);
    this.reloadTimer = setTimeout(() => this.reload(), 250);
  }

  provider(id: string): Provider | undefined {
    return this.providers().find((p) => p.id === id);
  }
}
