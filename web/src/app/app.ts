import { ChangeDetectionStrategy, Component, computed, inject, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';

import { Api } from './core/api';
import { Store } from './core/state';
import { Instance } from './core/models';
import { gigabytes } from './core/format';
import { Kanban } from './features/kanban/kanban';
import { InstanceDetail } from './features/instance-detail/instance-detail';
import { NewInstance } from './features/new-instance/new-instance';

@Component({
  selector: 'app-root',
  imports: [FormsModule, Kanban, InstanceDetail, NewInstance],
  templateUrl: './app.html',
  styleUrl: './app.css',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class App {
  readonly store = inject(Store);
  private readonly api = inject(Api);

  readonly detailFor = signal<string | null>(null);
  readonly creating = signal(false);

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

  readonly dockerLabel = computed(() => {
    const s = this.system();
    if (!s) return '';
    return s.dockerVersion ? `docker ${s.dockerVersion}` : 'docker fora do ar';
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
    this.detailFor.set(name);
  }

  setFilter(game: string | null): void {
    this.store.gameFilter.set(this.store.gameFilter() === game ? null : game);
  }
}
