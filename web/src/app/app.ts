import { ChangeDetectionStrategy, Component, computed, inject, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';

import { Api } from './core/api';
import { Store } from './core/state';
import { Prefs } from './core/prefs';
import { I18n } from './core/i18n/i18n';
import { Instance } from './core/models';
import { gigabytes } from './core/format';
import { Kanban } from './features/kanban/kanban';
import { InstanceDetail } from './features/instance-detail/instance-detail';
import { NewInstance } from './features/new-instance/new-instance';
import { Settings } from './features/settings/settings';

@Component({
  selector: 'app-root',
  imports: [FormsModule, Kanban, InstanceDetail, NewInstance, Settings],
  templateUrl: './app.html',
  styleUrl: './app.css',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class App {
  readonly store = inject(Store);
  readonly prefs = inject(Prefs);
  private readonly i18n = inject(I18n);
  private readonly api = inject(Api);

  readonly t = this.i18n.t;

  readonly detailFor = signal<string | null>(null);
  readonly creating = signal(false);
  readonly settingsOpen = signal(false);

  readonly system = computed(() => this.store.system());

  readonly memoryLabel = computed(() => {
    const s = this.system();
    if (!s) return '—';
    return `${gigabytes(s.memoryUsed)}/${gigabytes(s.memoryTotal)} GB`;
  });

  readonly diskLabel = computed(() => {
    const s = this.system();
    if (!s) return '—';
    return `${gigabytes(s.diskUsed)}/${gigabytes(s.diskTotal)} GB`;
  });

  readonly budgetLabel = computed(() => {
    const s = this.system();
    if (!s) return '';
    return `${gigabytes(s.memoryCommitted)}/${gigabytes(s.memoryBudget)} GB`;
  });

  readonly budgetPct = computed(() => {
    const s = this.system();
    if (!s || !s.memoryBudget) return 0;
    return Math.min(100, (s.memoryCommitted / s.memoryBudget) * 100);
  });

  readonly budgetColor = computed(() => {
    const p = this.budgetPct();
    if (p >= 90) return 'var(--bad)';
    if (p >= 70) return 'var(--warn)';
    return 'var(--ok)';
  });

  readonly dnsLabel = computed(() => {
    const d = this.store.dns();
    if (!d || !d.token) return this.t('app.dns.noToken');
    const failing = d.domains.filter((n) => n.lastError).length;
    if (failing) return this.i18n.plural('app.dns.failing', failing);
    if (!d.domains.length) return this.t('app.dns.noNames');
    return this.i18n.plural('app.dns.names', d.domains.length);
  });

  readonly dnsState = computed<'ok' | 'bad' | 'off'>(() => {
    const d = this.store.dns();
    if (!d || !d.token || !d.domains.length) return 'off';
    return d.domains.some((n) => n.lastError) ? 'bad' : 'ok';
  });

  readonly dnsTitle = computed(() => {
    const d = this.store.dns();
    if (!d || !d.token) return this.t('app.dns.titleNoToken');
    const failed = d.domains.filter((n) => n.lastError);
    if (failed.length) return failed.map((n) => `${n.hostname}: ${n.lastError}`).join('\n');
    if (!d.domains.length) return this.t('app.dns.titleNoNames');
    return d.domains.map((n) => `${n.hostname} → ${n.lastIp || '—'}`).join('\n');
  });

  readonly apiError = computed(() => this.api.lastError());

  constructor() {
    this.store.start();
  }

  openDetail(instance: Instance): void {
    this.detailFor.set(instance.name);
  }

  onCreated(name: string): void {
    this.creating.set(false);
    this.store.reload();
    this.store.notify(this.t('app.created', { name }));
  }

  setFilter(game: string | null): void {
    this.store.gameFilter.set(this.store.gameFilter() === game ? null : game);
  }
}
