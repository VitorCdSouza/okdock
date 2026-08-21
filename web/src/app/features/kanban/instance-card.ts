import { ChangeDetectionStrategy, Component, computed, input, output, signal } from '@angular/core';

import { Instance, STATE_META } from '../../core/models';
import { bytes, since } from '../../core/format';

const GAME_COLORS: Record<string, { bg: string; fg: string }> = {
  'minecraft-java': { bg: '#1c3323', fg: '#4fd99b' },
  'minecraft-bedrock': { bg: '#1c3323', fg: '#4fd99b' },
  terraria: { bg: '#33261c', fg: '#e5b567' },
  palworld: { bg: '#1e2f43', fg: '#6aa6f5' },
  valheim: { bg: '#2b2438', fg: '#b79cf0' },
  ark: { bg: '#33261c', fg: '#e08a5a' },
  factorio: { bg: '#332a1c', fg: '#e5b567' },
  satisfactory: { bg: '#1e2f43', fg: '#7fb2f0' },
  custom: { bg: '#1e222b', fg: '#9297a3' },
};

type Action = { label: string; kind: 'go' | 'bad' | 'flat'; verb: ActionVerb };
export type ActionVerb = 'start' | 'stop' | 'restart' | 'logs' | 'fix' | 'unarchive' | 'cancel';

@Component({
  selector: 'gd-instance-card',
  templateUrl: './instance-card.html',
  styleUrl: './instance-card.css',
  changeDetection: ChangeDetectionStrategy.OnPush,
  host: {
    '(document:click)': 'menuOpen.set(false)',
    '(document:keydown.escape)': 'menuOpen.set(false)',
  },
})
export class InstanceCard {
  readonly instance = input.required<Instance>();
  readonly short = input('··');

  readonly open = output<Instance>();
  readonly act = output<{ instance: Instance; verb: ActionVerb }>();
  readonly remove = output<Instance>();

  readonly menuOpen = signal(false);

  toggleMenu(event: Event): void {
    event.stopPropagation();
    this.menuOpen.update((v) => !v);
  }

  pickRemove(event: Event): void {
    event.stopPropagation();
    this.menuOpen.set(false);
    this.remove.emit(this.instance());
  }

  readonly colors = computed(() => GAME_COLORS[this.instance().game] ?? GAME_COLORS['custom']);
  readonly dot = computed(() => STATE_META[this.instance().state].dot);

  readonly tags = computed(() => {
    const i = this.instance();
    const out = i.ports.slice(0, 2).map((p) => `:${p.host}${p.protocol === 'udp' ? '/udp' : ''}`);
    if (i.memoryLimit) out.push(i.memoryLimit.toUpperCase());
    return out;
  });

  readonly action = computed<Action>(() => {
    switch (this.instance().state) {
      case 'stopped':
        return { label: '▶ Iniciar', kind: 'go', verb: 'start' };
      case 'running':
        return { label: '■ Parar', kind: 'bad', verb: 'stop' };
      case 'starting':
        return { label: 'Logs', kind: 'flat', verb: 'logs' };
      case 'error':
        return { label: 'Corrigir', kind: 'bad', verb: 'fix' };
      case 'archived':
        return { label: 'Restaurar', kind: 'flat', verb: 'unarchive' };
      default:
        return { label: 'Detalhes', kind: 'flat', verb: 'logs' };
    }
  });

  readonly meta = computed(() => {
    const i = this.instance();
    if (i.operation?.error) return i.operation.error;
    if (i.state === 'error') return i.status || `saiu com código ${i.exitCode ?? '?'}`;
    if (i.status) return i.status;
    if (i.state === 'archived') return `arquivada ${since(i.updatedAt)}`;
    return `parada ${since(i.updatedAt)}`;
  });

  readonly metaColor = computed(() => {
    const s = this.instance().state;
    if (s === 'error') return 'var(--bad)';
    if (s === 'running') return 'var(--ok)';
    if (s === 'updating') return 'var(--busy)';
    return 'var(--fg-dim)';
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
