import { ChangeDetectionStrategy, Component, computed, inject, output, signal } from '@angular/core';

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

  readonly pendingDelete = signal<Instance | null>(null);
  readonly deleteData = signal(false);
  readonly deleting = signal(false);

  askRemove(instance: Instance): void {
    this.deleteData.set(false);
    this.pendingDelete.set(instance);
  }

  cancelRemove(): void {
    this.pendingDelete.set(null);
  }

  confirmRemove(): void {
    const target = this.pendingDelete();
    if (!target) return;
    this.deleting.set(true);
    this.api.remove(target.name, !this.deleteData()).subscribe({
      next: () => {
        this.deleting.set(false);
        this.pendingDelete.set(null);
        this.store.reload();
      },
      error: () => {
        this.deleting.set(false);
        this.pendingDelete.set(null);
      },
    });
  }

  readonly columns = computed(() =>
    this.store.states().map((state) => ({
      state,
      ...STATE_META[state],
      cards: this.store.byState(state),
    })),
  );

  readonly total = computed(() => this.store.filtered().length);

  readonly dropTarget = signal<State | null>(null);
  readonly pendingAction = signal<{ instance: Instance; target: State } | null>(null);
  readonly acting = signal(false);

  private readonly dragged = computed(() => {
    const name = this.store.dragging();
    return name ? this.store.instances().find((i) => i.name === name) : undefined;
  });

  canDrop(target: State): boolean {
    const inst = this.dragged();
    if (!inst) return false;
    switch (target) {
      case 'updating':
        return !inst.archived && inst.state !== 'updating';
      case 'stopped':
        return this.isUp(inst.state);
      default:
        return false;
    }
  }

  private isUp(state: State): boolean {
    return state === 'running' || state === 'starting' || state === 'provisioning' || state === 'updating';
  }

  onDragChange(name: string | null): void {
    this.store.dragging.set(name);
    if (!name) this.dropTarget.set(null);
  }

  onDragOver(event: DragEvent, target: State): void {
    if (!this.canDrop(target)) return;
    event.preventDefault();
    if (event.dataTransfer) event.dataTransfer.dropEffect = 'move';
    this.dropTarget.set(target);
  }

  onDragLeave(target: State): void {
    if (this.dropTarget() === target) this.dropTarget.set(null);
  }

  onDrop(event: DragEvent, target: State): void {
    event.preventDefault();
    this.dropTarget.set(null);
    const inst = this.dragged();
    this.store.dragging.set(null);
    if (!inst || !this.canDrop(target)) return;
    this.pendingAction.set({ instance: inst, target });
  }

  cancelAction(): void {
    this.pendingAction.set(null);
  }

  confirmAction(): void {
    const pending = this.pendingAction();
    if (!pending) return;
    this.acting.set(true);
    const call =
      pending.target === 'updating'
        ? this.api.updateImage(pending.instance.name)
        : this.api.stop(pending.instance.name);
    call.subscribe({
      next: () => {
        this.acting.set(false);
        this.pendingAction.set(null);
        this.store.reload();
      },
      error: () => {
        this.acting.set(false);
        this.pendingAction.set(null);
      },
    });
  }

  short(i: Instance): string {
    return this.store.provider(i.providerId)?.short ?? '··';
  }

  visible(state: State, count: number): boolean {
    if (count > 0) return true;
    switch (state) {
      case 'stopped':
      case 'running':
      case 'updating':
      case 'error':
        return true;
      default:
        return false;
    }
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
