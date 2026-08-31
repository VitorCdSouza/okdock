import { ChangeDetectionStrategy, Component, computed, inject, input, output, signal } from '@angular/core';

import { Instance, STATE_DOT } from '../../core/models';
import { I18n } from '../../core/i18n/i18n';
import { bytes } from '../../core/format';
import { copyText } from '../../core/clipboard';
import { TemplateIcon, templateColors } from '../../shared/template-icon';

type Action = { label: string; kind: 'go' | 'bad' | 'flat'; verb: ActionVerb };
export type ActionVerb = 'start' | 'stop' | 'restart' | 'logs' | 'fix' | 'unarchive' | 'cancel';

@Component({
  selector: 'ok-instance-card',
  imports: [TemplateIcon],
  templateUrl: './instance-card.html',
  styleUrl: './instance-card.css',
  changeDetection: ChangeDetectionStrategy.OnPush,
  host: {
    '(document:click)': 'menuOpen.set(false)',
    '(document:keydown.escape)': 'menuOpen.set(false)',
  },
})
export class InstanceCard {
  private readonly i18n = inject(I18n);
  readonly t = this.i18n.t;

  readonly instance = input.required<Instance>();
  readonly short = input('··');
  readonly inStack = input(false);

  readonly open = output<Instance>();
  readonly copied = output<string>();
  readonly act = output<{ instance: Instance; verb: ActionVerb }>();
  readonly remove = output<Instance>();
  readonly dragChange = output<string | null>();

  readonly menuOpen = signal(false);

  // inside a stack the project is the group title, and one named after the container says nothing
  readonly showProject = computed(() => {
    const i = this.instance();
    if (!i.external || this.inStack()) return false;
    return i.project !== i.name;
  });

  readonly address = computed(() => {
    const i = this.instance();
    const port = (i.ports ?? [])[0]?.host;
    if (!i.dns || !port) return i.dns?.hostname ?? '';
    return `${i.dns.hostname}:${port}`;
  });

  copyAddress(event: Event): void {
    event.stopPropagation();
    const addr = this.address();
    if (!addr) return;
    copyText(addr);
    this.copied.emit(addr);
  }

  toggleMenu(event: Event): void {
    event.stopPropagation();
    this.menuOpen.update((v) => !v);
  }

  pickRemove(event: Event): void {
    event.stopPropagation();
    this.menuOpen.set(false);
    this.remove.emit(this.instance());
  }

  pickEdit(event: Event): void {
    event.stopPropagation();
    this.menuOpen.set(false);
    this.open.emit(this.instance());
  }

  onDragStart(event: DragEvent): void {
    event.dataTransfer?.setData('text/plain', this.instance().name);
    if (event.dataTransfer) event.dataTransfer.effectAllowed = 'move';
    this.dragChange.emit(this.instance().name);
  }

  onDragEnd(): void {
    this.dragChange.emit(null);
  }

  readonly colors = computed(() =>
    templateColors(this.instance().templateId, this.instance().category),
  );
  readonly dot = computed(() => STATE_DOT[this.instance().state]);

  readonly portList = computed(() => {
    const ports = this.instance().ports ?? [];
    if (!ports.length) return '-';
    return ports.map((p) => `${p.host}${p.protocol === 'udp' ? '/udp' : ''}`).join(', ');
  });

  readonly portCount = computed(() => (this.instance().ports ?? []).length);

  readonly memoryAlloc = computed(() =>
    memoryLabel(this.instance().memoryLimit, this.t('card.noLimit')),
  );

  readonly action = computed<Action>(() => {
    switch (this.instance().state) {
      case 'stopped':
        return { label: this.t('card.action.start'), kind: 'go', verb: 'start' };
      case 'running':
        return { label: this.t('card.action.stop'), kind: 'bad', verb: 'stop' };
      case 'starting':
        return { label: this.t('card.action.logs'), kind: 'flat', verb: 'logs' };
      case 'error':
        return { label: this.t('card.action.fix'), kind: 'bad', verb: 'fix' };
      case 'archived':
        return { label: this.t('card.action.restore'), kind: 'flat', verb: 'unarchive' };
      default:
        return { label: this.t('card.action.details'), kind: 'flat', verb: 'logs' };
    }
  });

  readonly meta = computed(() => {
    const i = this.instance();
    if (i.operation?.error) return i.operation.error;
    if (i.state === 'error') return i.status || this.t('card.exited', { code: i.exitCode ?? '?' });
    if (i.status) return i.status;
    const when = this.i18n.since(i.updatedAt);
    if (i.state === 'archived') return this.t('card.archivedSince', { when });
    return this.t('card.stoppedSince', { when });
  });

  readonly metaColor = computed(() => {
    const s = this.instance().state;
    if (s === 'error') return 'var(--bad)';
    if (s === 'running') return 'var(--ok)';
    if (s === 'updating') return 'var(--busy)';
    return 'var(--fg-dim)';
  });

  readonly opLabel = computed(() => {
    const op = this.instance().operation;
    if (!op) return '';
    return op.code ? this.i18n.maybe(`op.${op.code}`) ?? op.code : op.message;
  });

  readonly memUsage = computed(() => {
    const s = this.instance().stats;
    return s ? bytes(s.memoryBytes) : '';
  });

  readonly cpuUsage = computed(() => {
    const s = this.instance().stats;
    return s ? `${s.cpuPercent.toFixed(0)}%` : '';
  });
}

function memoryLabel(limit: string | undefined, noLimit: string): string {
  if (!limit) return noLimit;
  const m = /^(\d+(?:\.\d+)?)\s*([gmk])?b?$/i.exec(limit.trim());
  if (!m) return limit;
  const unit = { g: 'GB', m: 'MB', k: 'KB', '': 'B' }[m[2]?.toLowerCase() ?? ''];
  return `${m[1]} ${unit}`;
}
