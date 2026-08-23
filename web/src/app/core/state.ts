import { Injectable, computed, inject, signal } from '@angular/core';

import { Api } from './api';
import { Events } from './events';
import { I18n } from './i18n/i18n';
import {
  COLUMN_OF,
  Category,
  DnsStatus,
  Instance,
  State,
  SystemInfo,
  Template,
} from './models';

@Injectable({ providedIn: 'root' })
export class Store {
  private readonly api = inject(Api);
  private readonly events = inject(Events);
  private readonly i18n = inject(I18n);

  readonly instances = signal<Instance[]>([]);
  readonly states = signal<State[]>([]);
  readonly templates = signal<Template[]>([]);
  readonly categories = signal<Category[]>([]);
  readonly system = signal<SystemInfo | null>(null);
  readonly dns = signal<DnsStatus | null>(null);
  readonly loading = signal(true);
  readonly categoryFilter = signal<Category | null>(null);
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
    const category = this.categoryFilter();
    return this.instances().filter((i) => {
      if (category && i.category !== category) return false;
      if (!term) return true;
      const haystack = [
        i.name,
        i.image,
        i.templateId,
        ...(i.ports ?? []).map((p) => String(p.host)),
      ]
        .join(' ')
        .toLowerCase();
      return haystack.includes(term);
    });
  });

  readonly categoryCounts = computed(() => {
    const counts = new Map<Category, number>();
    for (const i of this.instances()) {
      counts.set(i.category, (counts.get(i.category) ?? 0) + 1);
    }
    return this.categories()
      .filter((c) => counts.has(c))
      .map((category) => ({ category, count: counts.get(category)! }));
  });

  readonly byCategory = computed(() => {
    const groups = new Map<Category, Template[]>();
    for (const t of this.templates()) {
      groups.set(t.category, [...(groups.get(t.category) ?? []), t]);
    }
    return this.categories()
      .filter((c) => groups.has(c))
      .map((category) => ({ category, templates: groups.get(category)! }));
  });

  byColumn(column: State): Instance[] {
    return this.filtered().filter((i) => COLUMN_OF[i.state] === column);
  }

  start(): void {
    if (this.started) return;
    this.started = true;

    this.reloadTemplates();
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

  reloadTemplates(): void {
    this.api.templates().subscribe({
      next: ({ templates, categories }) => {
        this.templates.set(templates);
        this.categories.set(categories);
      },
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

  template(id: string): Template | undefined {
    return this.templates().find((t) => t.id === id);
  }
}
