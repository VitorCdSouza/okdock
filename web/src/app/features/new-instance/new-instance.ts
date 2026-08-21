import { ChangeDetectionStrategy, Component, computed, inject, output, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';

import { Api, GameDockError } from '../../core/api';
import { Store } from '../../core/state';
import { Provider, SpecRequest } from '../../core/models';
import { ProviderForm } from '../../shared/provider-form';

type Step = 1 | 2 | 3;

@Component({
  selector: 'gd-new-instance',
  imports: [FormsModule, ProviderForm],
  templateUrl: './new-instance.html',
  styleUrl: './new-instance.css',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class NewInstance {
  private readonly api = inject(Api);
  readonly store = inject(Store);

  readonly close = output<void>();
  readonly created = output<string>();

  readonly step = signal<Step>(1);
  readonly provider = signal<Provider | null>(null);
  readonly name = signal('');
  readonly image = signal('');
  readonly memoryLimit = signal('');
  readonly values = signal<Record<string, string>>({});
  readonly hostPorts = signal<Record<string, number>>({});
  readonly startAfterCreate = signal(true);

  readonly compose = signal('');
  readonly busy = signal(false);
  readonly error = signal<GameDockError | null>(null);

  readonly providers = computed(() => this.store.providers());

  readonly ports = computed(() => {
    const p = this.provider();
    if (!p) return [];
    return (p.ports ?? []).map((port) => ({
      ...port,
      host: this.hostPorts()[this.portKey(port.container, port.protocol)] ?? port.defaultHost,
    }));
  });

  readonly nameError = computed(() => {
    const n = this.name();
    if (!n) return '';
    if (!/^[a-z0-9][a-z0-9_-]{1,38}$/.test(n)) {
      return 'minúsculas, dígitos, - e _; 2 a 39 caracteres, começando por letra ou dígito';
    }
    if (this.store.instances().some((i) => i.name === n)) return 'já existe uma instância com esse nome';
    return '';
  });

  readonly canAdvance = computed(() => {
    if (this.step() === 1) return !!this.provider();
    if (this.step() === 2) return !!this.name() && !this.nameError() && !!this.provider();
    return true;
  });

  readonly budgetWarning = computed(() => {
    const sys = this.store.system();
    const p = this.provider();
    if (!sys || !p) return '';
    const want = parseMemory(this.memoryLimit() || p.defaultMemory);
    const free = sys.memoryBudget - sys.memoryCommitted;
    if (want > free) {
      return `Esta instância pede ${this.memoryLimit() || p.defaultMemory}, mas só há ` +
        `${(free / 1024 ** 3).toFixed(1)} GB livres no orçamento. Dá para criar parada e subir depois.`;
    }
    return '';
  });

  portKey(container: number, protocol: string): string {
    return `${container}/${protocol}`;
  }

  pick(p: Provider): void {
    this.provider.set(p);
    this.image.set(p.image);
    this.memoryLimit.set(p.defaultMemory);
    const defaults: Record<string, string> = {};
    for (const f of p.fields ?? []) {
      if (f.default) defaults[f.key] = f.default;
    }
    this.values.set(defaults);
    this.hostPorts.set({});
    this.error.set(null);
  }

  setPort(container: number, protocol: string, raw: string): void {
    const n = Number(raw);
    if (!Number.isFinite(n)) return;
    this.hostPorts.update((cur) => ({ ...cur, [this.portKey(container, protocol)]: n }));
  }

  next(): void {
    if (this.step() === 1) {
      this.step.set(2);
      return;
    }
    if (this.step() === 2) {
      this.preview();
      return;
    }
    this.submit();
  }

  back(): void {
    this.error.set(null);
    this.step.update((s) => (s > 1 ? ((s - 1) as Step) : s));
  }

  private preview(): void {
    this.busy.set(true);
    this.error.set(null);
    this.api.previewCompose(this.request()).subscribe({
      next: (res) => {
        this.compose.set(res.compose);
        this.step.set(3);
        this.busy.set(false);
      },
      error: (err: GameDockError) => {
        this.error.set(err);
        this.busy.set(false);
        if (err.status === 422) this.step.set(2);
      },
    });
  }

  private submit(): void {
    this.busy.set(true);
    this.error.set(null);
    this.api.create({ ...this.request(), start: this.startAfterCreate() }).subscribe({
      next: () => {
        this.busy.set(false);
        this.created.emit(this.name());
      },
      error: (err: GameDockError) => {
        this.error.set(err);
        this.busy.set(false);
      },
    });
  }

  private request(): SpecRequest {
    const p = this.provider()!;
    return {
      name: this.name(),
      providerId: p.id,
      image: this.image() || undefined,
      values: this.values(),
      ports: this.ports().map((port) => ({
        host: port.host,
        container: port.container,
        protocol: port.protocol,
        label: port.label,
      })),
      memoryLimit: this.memoryLimit() || undefined,
    };
  }
}

function parseMemory(s: string): number {
  const m = /^(\d+(?:\.\d+)?)\s*([gmk]?)b?$/i.exec(s.trim());
  if (!m) return 0;
  const mult = { g: 1024 ** 3, m: 1024 ** 2, k: 1024, '': 1 }[m[2].toLowerCase()] ?? 1;
  return Number(m[1]) * mult;
}
