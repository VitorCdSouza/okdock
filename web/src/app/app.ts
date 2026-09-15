import { ChangeDetectionStrategy, Component, computed, effect, inject, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';

import { Api } from './core/api';
import { Store } from './core/state';
import { Prefs } from './core/prefs';
import { I18n } from './core/i18n/i18n';
import { Category, Instance } from './core/models';
import { gigabytes } from './core/format';
import { Kanban } from './features/kanban/kanban';
import { InstanceDetail } from './features/instance-detail/instance-detail';
import { NewInstance } from './features/new-instance/new-instance';
import { Settings } from './features/settings/settings';
import { Templates } from './features/templates/templates';

@Component({
  selector: 'app-root',
  imports: [FormsModule, Kanban, InstanceDetail, NewInstance, Settings, Templates],
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
  // the name a rename is heading to, which only exists once the folder finishes moving
  private readonly awaiting = signal<string | null>(null);
  readonly creating = signal(false);
  readonly settingsOpen = signal(false);
  readonly templatesOpen = signal(false);

  readonly system = computed(() => this.store.system());

  readonly memoryLabel = computed(() => {
    const s = this.system();
    if (!s) return '-';
    return `${gigabytes(s.memoryUsed)}/${gigabytes(s.memoryTotal)} GB`;
  });

  readonly diskLabel = computed(() => {
    const s = this.system();
    if (!s) return '-';
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

  readonly apiError = computed(() => this.api.lastError());

  constructor() {
    this.store.start();
    effect(() => {
      const name = this.awaiting();
      if (!name || !this.store.instances().some((i) => i.name === name)) return;
      this.awaiting.set(null);
      this.detailFor.set(name);
    });
  }

  openDetail(instance: Instance): void {
    this.detailFor.set(instance.name);
  }

  // the old name answers until the folder lands, so the open screen waits for the new one to show up
  onRenamed(name: string): void {
    this.awaiting.set(name);
    this.store.notify(this.t('app.renamed', { name }));
  }

  closeDetail(): void {
    this.detailFor.set(null);
    this.awaiting.set(null);
  }

  onCreated(name: string): void {
    this.creating.set(false);
    this.store.reload();
    this.store.notify(this.t('app.created', { name }));
  }

  setFilter(category: Category | null): void {
    this.store.categoryFilter.set(this.store.categoryFilter() === category ? null : category);
  }

  categoryName(category: Category): string {
    return this.i18n.category(category);
  }
}
