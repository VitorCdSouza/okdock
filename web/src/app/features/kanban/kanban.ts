import { ChangeDetectionStrategy, Component, computed, inject, output, signal } from '@angular/core';

import { Api } from '../../core/api';
import { Store } from '../../core/state';
import { COLUMN_OF, Instance, STATE_DOT, STATE_KEY, State } from '../../core/models';
import { I18n } from '../../core/i18n/i18n';
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

  readonly t = inject(I18n).t;

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

  // estado que nao e a propria coluna (provisionando, iniciando) entra na coluna de destino
  readonly columns = computed(() =>
    this.store
      .states()
      .filter((state) => COLUMN_OF[state] === state)
      .map((state) => ({
        state,
        dot: STATE_DOT[state],
        title: this.t(STATE_KEY[state]),
        cards: this.store.byColumn(state),
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

  allows(inst: Instance, target: State): boolean {
    // container externo nao tem Spec: da para subir e parar, nao para atualizar nem arquivar
    if (inst.external && target !== 'running' && target !== 'stopped') return false;
    switch (target) {
      case 'updating':
        return !inst.archived && inst.state !== 'updating';
      case 'stopped':
        return this.isUp(inst.state);
      case 'running':
        return inst.state === 'stopped' || inst.state === 'error';
      case 'archived':
        return !inst.archived;
      default:
        return false;
    }
  }

  canDrop(target: State): boolean {
    const inst = this.dragged();
    return !!inst && this.allows(inst, target);
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
    const allowed = !!inst && this.allows(inst, target);

    this.store.dragging.set(null);
    if (!inst || !allowed) return;
    this.pendingAction.set({ instance: inst, target });
  }

  cancelAction(): void {
    this.pendingAction.set(null);
  }

  confirmAction(): void {
    const pending = this.pendingAction();
    if (!pending) return;
    this.acting.set(true);
    this.call(pending.target, pending.instance.name).subscribe({
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

  dropHint(target: State): string {
    switch (target) {
      case 'updating':
        return this.t('kanban.dropToUpdate');
      case 'running':
        return this.t('kanban.dropToStart');
      case 'archived':
        return this.t('kanban.dropToArchive');
      default:
        return this.t('kanban.dropToStop');
    }
  }

  confirmLabel(target: State): string {
    switch (target) {
      case 'updating':
        return this.t('kanban.doUpdate');
      case 'running':
        return this.t('kanban.doStart');
      case 'archived':
        return this.t('kanban.doArchive');
      default:
        return this.t('kanban.doStop');
    }
  }

  private call(target: State, name: string) {
    switch (target) {
      case 'updating':
        return this.api.updateImage(name);
      case 'running':
        return this.api.start(name);
      case 'archived':
        return this.api.archive(name);
      default:
        return this.api.stop(name);
    }
  }

  short(i: Instance): string {
    return this.store.template(i.templateId)?.short ?? '··';
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
