import {
  ChangeDetectionStrategy,
  Component,
  DestroyRef,
  computed,
  effect,
  inject,
  input,
  output,
  signal,
} from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { FormsModule } from '@angular/forms';

import { Api, GameDockError } from '../../core/api';
import { Events } from '../../core/events';
import { Store } from '../../core/state';
import { Instance, SpecRequest, State } from '../../core/models';
import { ProviderForm } from '../../shared/provider-form';
import { bytes, since } from '../../core/format';

type Tab = 'config' | 'console' | 'compose' | 'recursos';

const STATE_CHIP: Record<State, { label: string; bg: string; line: string; fg: string }> = {
  running: { label: 'RODANDO', bg: 'var(--ok-bg)', line: 'var(--ok-line)', fg: 'var(--ok)' },
  starting: { label: 'INICIANDO', bg: 'var(--warn-bg)', line: 'var(--warn-line)', fg: 'var(--warn)' },
  provisioning: { label: 'PROVISIONANDO', bg: '#111b2b', line: '#2c3a4f', fg: 'var(--accent)' },
  updating: { label: 'ATUALIZANDO', bg: 'var(--busy-bg)', line: 'var(--busy-line)', fg: 'var(--busy)' },
  stopped: { label: 'PARADO', bg: '#1a1d24', line: 'var(--line-strong)', fg: 'var(--fg-muted)' },
  error: { label: 'ERRO', bg: 'var(--bad-bg)', line: 'var(--bad-line)', fg: 'var(--bad)' },
  archived: { label: 'ARQUIVADO', bg: '#141519', line: 'var(--line)', fg: 'var(--fg-faint)' },
};

@Component({
  selector: 'gd-instance-detail',
  imports: [FormsModule, ProviderForm],
  templateUrl: './instance-detail.html',
  styleUrl: './instance-detail.css',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class InstanceDetail {
  private readonly api = inject(Api);
  private readonly events = inject(Events);
  private readonly destroyRef = inject(DestroyRef);
  readonly store = inject(Store);

  readonly name = input.required<string>();
  readonly close = output<void>();

  readonly tab = signal<Tab>('config');
  readonly values = signal<Record<string, string>>({});
  readonly memoryLimit = signal('');
  readonly cpus = signal(0);
  readonly hostPorts = signal<Record<string, number>>({});

  readonly preview = signal('');
  readonly recreate = signal<string[]>([]);
  readonly rawCompose = signal('');
  readonly logLines = signal<string[]>([]);
  readonly busy = signal(false);
  readonly error = signal<GameDockError | null>(null);

  readonly instance = computed<Instance | undefined>(() =>
    this.store.instances().find((i) => i.name === this.name()),
  );

  readonly provider = computed(() => {
    const i = this.instance();
    return i ? this.store.provider(i.providerId) : undefined;
  });

  readonly chip = computed(() => STATE_CHIP[this.instance()?.state ?? 'stopped']);

  readonly isUp = computed(() => {
    const s = this.instance()?.state;
    return s === 'running' || s === 'starting' || s === 'updating' || s === 'provisioning';
  });

  readonly subtitle = computed(() => {
    const i = this.instance();
    if (!i) return '';
    const parts = [i.image];
    if (i.status) parts.push(i.status);
    parts.push(i.dir);
    return parts.join(' · ');
  });

  readonly statsLine = computed(() => {
    const s = this.instance()?.stats;
    if (!s) return '';
    return `${s.cpuPercent.toFixed(0)}% CPU · ${bytes(s.memoryBytes)} de ${bytes(s.memoryLimit)}`;
  });

  readonly createdLabel = computed(() => {
    const i = this.instance();
    return i ? `criada ${since(i.createdAt)}` : '';
  });

  private loadedFor = '';

  constructor() {
    effect(() => {
      const i = this.instance();
      if (!i || this.loadedFor === i.name) return;
      this.loadedFor = i.name;
      this.values.set({ ...i.env });
      this.memoryLimit.set(i.memoryLimit);
      this.cpus.set(i.cpus);
      this.hostPorts.set(
        Object.fromEntries(i.ports.map((p) => [`${p.container}/${p.protocol}`, p.host])),
      );
      this.error.set(null);
      this.refreshPreview();
    });
  }

  select(tab: Tab): void {
    this.tab.set(tab);
    if (tab === 'compose' && !this.rawCompose()) {
      this.api.compose(this.name()).subscribe({
        next: (raw) => this.rawCompose.set(raw),
        error: () => {},
      });
    }
    if (tab === 'console' && this.logLines().length === 0) {
      this.tailLogs();
    }
  }

  private tailLogs(): void {
    this.logLines.set([]);
    this.events
      .logs(this.name(), 300)
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe({
        next: (line) =>
          this.logLines.update((cur) => (cur.length > 500 ? [...cur.slice(-400), line] : [...cur, line])),
        error: () => {},
      });
  }

  refreshPreview(): void {
    const req = this.request();
    if (!req) return;
    this.api.previewCompose(req).subscribe({
      next: (res) => {
        this.preview.set(res.compose);
        this.recreate.set(res.recreate ?? []);
      },
      error: () => {},
    });
  }

  setPort(container: number, protocol: string, raw: string): void {
    const n = Number(raw);
    if (!Number.isFinite(n)) return;
    this.hostPorts.update((cur) => ({ ...cur, [`${container}/${protocol}`]: n }));
    this.refreshPreview();
  }

  save(): void {
    const req = this.request();
    if (!req) return;
    this.busy.set(true);
    this.error.set(null);
    this.api.update(this.name(), req).subscribe({
      next: () => {
        this.busy.set(false);
        this.store.reload();
      },
      error: (err: GameDockError) => {
        this.error.set(err);
        this.busy.set(false);
      },
    });
  }

  discard(): void {
    this.loadedFor = '';
    this.error.set(null);
    this.store.reload();
  }

  start(): void {
    this.run(this.api.start(this.name()));
  }

  stop(): void {
    this.run(this.api.stop(this.name()));
  }

  restart(): void {
    this.run(this.api.restart(this.name()));
  }

  archive(): void {
    this.run(this.api.archive(this.name()));
  }

  unarchive(): void {
    this.run(this.api.unarchive(this.name()));
  }

  clearError(): void {
    this.run(this.api.clearError(this.name()));
  }

  private run(obs: { subscribe: (o: object) => unknown }): void {
    this.busy.set(true);
    this.error.set(null);
    obs.subscribe({
      next: () => {
        this.busy.set(false);
        this.store.reload();
      },
      error: (err: GameDockError) => {
        this.error.set(err);
        this.busy.set(false);
      },
    });
  }

  private request(): SpecRequest | null {
    const i = this.instance();
    if (!i) return null;
    return {
      name: i.name,
      providerId: i.providerId,
      image: i.image,
      values: this.values(),
      ports: i.ports.map((p) => ({
        host: this.hostPorts()[`${p.container}/${p.protocol}`] ?? p.host,
        container: p.container,
        protocol: p.protocol,
        label: p.label,
      })),
      memoryLimit: this.memoryLimit(),
      cpus: this.cpus(),
      restart: i.restart,
    };
  }
}
