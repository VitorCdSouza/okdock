import {
  ChangeDetectionStrategy,
  Component,
  ElementRef,
  afterRenderEffect,
  computed,
  inject,
  output,
  signal,
  viewChild,
  viewChildren,
} from '@angular/core';
import { Observable, concat } from 'rxjs';

import { Api, OkDockError } from '../../core/api';
import { Store } from '../../core/state';
import { COLUMN_OF, Instance, STATE_DOT, STATE_KEY, State } from '../../core/models';
import { I18n } from '../../core/i18n/i18n';
import { TemplateIcon, templateColors } from '../../shared/template-icon';
import { ActionVerb, InstanceCard } from './instance-card';

type HiddenColumn = { title: string; count: number };
type MiniIcon = { name: string; templateId: string; short: string; bg: string; fg: string };
type CardItem = { kind: 'card'; key: string; instance: Instance };
type GroupItem = {
  kind: 'group';
  key: string;
  name: string;
  members: Instance[];
  icons: MiniIcon[];
  summary: string;
  open: boolean;
};
type Item = CardItem | GroupItem;
type DragGroup = { name: string; members: Instance[] };
type Pending = { name: string; instances: Instance[]; target: State };

@Component({
  selector: 'ok-kanban',
  imports: [TemplateIcon, InstanceCard],
  templateUrl: './kanban.html',
  styleUrl: './kanban.css',
  changeDetection: ChangeDetectionStrategy.OnPush,
  host: {
    '(window:resize)': 'measure()',
    '(document:keydown.escape)': 'closeGroups()',
  },
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
      error: (err: OkDockError) => {
        this.deleting.set(false);
        this.pendingDelete.set(null);
        this.store.notifyError(err.message);
      },
    });
  }

  readonly openGroups = signal<ReadonlySet<string>>(new Set());

  toggleGroup(key: string): void {
    this.openGroups.update((current) => {
      const next = new Set(current);
      if (!next.delete(key)) next.add(key);
      return next;
    });
  }

  closeGroups(): void {
    this.openGroups.set(new Set());
  }

  // the two columns that hold the day to day are twice as wide, whatever they have in them
  private static readonly WIDE: ReadonlySet<State> = new Set<State>(['stopped', 'running']);

  readonly columns = computed(() => {
    const opened = this.openGroups();
    return this.store
      .states()
      .filter((state) => COLUMN_OF[state] === state)
      .map((state) => {
        const cards = this.store.byColumn(state);
        return {
          state,
          dot: STATE_DOT[state],
          title: this.t(STATE_KEY[state]),
          cards,
          items: this.pack(state, cards, opened),
          grow: Kanban.WIDE.has(state) ? 2 : 1,
        };
      });
  });

  // containers of the same compose stack collapse into one tile, opened one at a time
  private pack(state: State, cards: Instance[], opened: ReadonlySet<string>): Item[] {
    const stacks = new Map<string, Instance[]>();
    for (const card of cards) {
      if (!card.project) continue;
      stacks.set(card.project, [...(stacks.get(card.project) ?? []), card]);
    }

    const groups: GroupItem[] = [];
    const loose: CardItem[] = [];
    const done = new Set<string>();
    for (const card of cards) {
      const project = card.project;
      const members = project ? stacks.get(project) ?? [] : [];
      if (!project || members.length < 2) {
        loose.push({ kind: 'card', key: card.name, instance: card });
        continue;
      }
      if (done.has(project)) continue;
      done.add(project);
      const key = `${state}:${project}`;
      groups.push({
        kind: 'group',
        key,
        name: project,
        members,
        icons: members.slice(0, 4).map((m) => this.icon(m)),
        summary: members.map((m) => m.name).join(', '),
        open: opened.has(key),
      });
    }
    // the tiles go on top, so an open group never pushes the loose cards around
    return [...groups, ...loose];
  }

  private icon(instance: Instance): MiniIcon {
    const { bg, fg } = templateColors(instance.templateId, instance.category);
    return { name: instance.name, templateId: instance.templateId, short: this.short(instance), bg, fg };
  }

  private readonly board = viewChild<ElementRef<HTMLElement>>('board');
  private readonly columnEls = viewChildren<ElementRef<HTMLElement>>('columnEl');

  readonly hiddenLeft = signal<HiddenColumn[]>([]);
  readonly hiddenRight = signal<HiddenColumn[]>([]);

  constructor() {
    afterRenderEffect(() => {
      this.columns();
      this.measure();
    });
  }

  // which columns fell outside the visible part of the board
  measure(): void {
    const board = this.board()?.nativeElement;
    if (!board) return;

    const view = board.getBoundingClientRect();
    const columns = this.columns();
    const left: HiddenColumn[] = [];
    const right: HiddenColumn[] = [];

    this.columnEls().forEach((ref, i) => {
      const col = columns[i];
      if (!col) return;
      const box = ref.nativeElement.getBoundingClientRect();
      const hidden = { title: col.title, count: col.cards.length };
      if (box.right > view.right + 1) right.push(hidden);
      else if (box.left < view.left - 1) left.push(hidden);
    });

    this.hiddenLeft.set(left);
    this.hiddenRight.set(right);
  }

  scrollBoard(direction: 1 | -1): void {
    const board = this.board()?.nativeElement;
    if (!board) return;
    board.scrollBy({ left: direction * board.clientWidth * 0.7, behavior: 'smooth' });
  }

  readonly total = computed(() => this.store.filtered().length);

  readonly dropTarget = signal<State | null>(null);
  readonly pendingAction = signal<Pending | null>(null);
  readonly acting = signal(false);

  readonly draggingGroup = signal<DragGroup | null>(null);

  private readonly dragged = computed(() => {
    const name = this.store.dragging();
    return name ? this.store.instances().find((i) => i.name === name) : undefined;
  });

  allows(inst: Instance, target: State): boolean {
    // updating needs the compose file the panel can write
    if (inst.external && target === 'updating' && !inst.editable) return false;
    switch (target) {
      case 'updating':
        return inst.state !== 'updating';
      case 'stopped':
        return this.isUp(inst.state);
      case 'running':
        return inst.state === 'stopped' || inst.state === 'error';
      default:
        return false;
    }
  }

  canDrop(target: State): boolean {
    return this.moving(target).length > 0;
  }

  // a stack drops as a whole, minus the members the column refuses
  private moving(target: State): Instance[] {
    const group = this.draggingGroup();
    if (group) return group.members.filter((inst) => this.allows(inst, target));
    const inst = this.dragged();
    return inst && this.allows(inst, target) ? [inst] : [];
  }

  private isUp(state: State): boolean {
    return state === 'running' || state === 'starting' || state === 'provisioning' || state === 'updating';
  }

  onDragChange(name: string | null): void {
    this.store.dragging.set(name);
    if (!name) this.dropTarget.set(null);
  }

  onGroupDragStart(event: DragEvent, item: GroupItem): void {
    event.dataTransfer?.setData('text/plain', item.name);
    if (event.dataTransfer) event.dataTransfer.effectAllowed = 'move';
    this.store.dragging.set(null);
    this.draggingGroup.set({ name: item.name, members: item.members });
  }

  onGroupDragEnd(): void {
    this.draggingGroup.set(null);
    this.dropTarget.set(null);
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

    const moving = this.moving(target);
    const name = this.draggingGroup()?.name ?? moving[0]?.name ?? '';

    this.store.dragging.set(null);
    this.draggingGroup.set(null);
    if (!moving.length) return;
    this.pendingAction.set({ name, instances: moving, target });
  }

  anyRunning(instances: Instance[]): boolean {
    return instances.some((inst) => inst.state === 'running');
  }

  memberNames(instances: Instance[]): string {
    return instances.map((inst) => inst.name).join(', ');
  }

  cancelAction(): void {
    this.pendingAction.set(null);
  }

  confirmAction(): void {
    const pending = this.pendingAction();
    if (!pending) return;
    this.acting.set(true);
    // one at a time, because the budget of an instance is checked against what is already up
    const calls = pending.instances.map((inst) => this.call(pending.target, inst.name));
    concat(...calls).subscribe({
      complete: () => {
        this.acting.set(false);
        this.pendingAction.set(null);
        this.store.reload();
      },
      error: (err: OkDockError) => {
        this.acting.set(false);
        this.pendingAction.set(null);
        this.store.notifyError(err.message);
        this.store.reload();
      },
    });
  }

  dropHint(target: State): string {
    switch (target) {
      case 'updating':
        return this.t('kanban.dropToUpdate');
      case 'running':
        return this.t('kanban.dropToStart');
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
        this.fire(this.api.start(instance.name));
        break;
      case 'stop':
      case 'cancel':
        this.fire(this.api.stop(instance.name));
        break;
      case 'restart':
        this.fire(this.api.restart(instance.name));
        break;
      case 'fix':
      case 'logs':
        this.open.emit(instance);
        break;
    }
  }

  private fire(call: Observable<void>): void {
    call.subscribe({
      next: () => this.store.reload(),
      error: (err: OkDockError) => this.store.notifyError(err.message),
    });
  }
}
