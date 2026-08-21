import { ChangeDetectionStrategy, Component, computed, inject, output } from '@angular/core';

import { Api } from '../../core/api';
import { Store } from '../../core/state';
import { Instance, STATE_META, State } from '../../core/models';
import { ActionVerb, InstanceCard } from './instance-card';

@Component({
  selector: 'gd-kanban',
  imports: [InstanceCard],
  templateUrl: './kanban.html',
  styleUrl: './kanban.css',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class Kanban {
  private readonly api = inject(Api);
  readonly store = inject(Store);

  readonly open = output<Instance>();
  readonly create = output<void>();

  readonly columns = computed(() =>
    this.store.states().map((state) => ({
      state,
      ...STATE_META[state],
      cards: this.store.byState(state),
    })),
  );

  readonly total = computed(() => this.store.filtered().length);

  short(i: Instance): string {
    return this.store.provider(i.providerId)?.short ?? '··';
  }

  visible(state: State, count: number): boolean {
    return count > 0 || state === 'stopped' || state === 'running' || state === 'error';
  }

  onAction({ instance, verb }: { instance: Instance; verb: ActionVerb }): void {
    switch (verb) {
      case 'start':
        this.api.start(instance.name).subscribe({ error: () => {} });
        break;
      case 'stop':
        this.api.stop(instance.name).subscribe({ error: () => {} });
        break;
      case 'restart':
        this.api.restart(instance.name).subscribe({ error: () => {} });
        break;
      case 'unarchive':
        this.api.unarchive(instance.name).subscribe({ error: () => {} });
        break;
      case 'fix':
      case 'logs':
        this.open.emit(instance);
        break;
      case 'cancel':
        this.api.stop(instance.name).subscribe({ error: () => {} });
        break;
    }
  }
}
