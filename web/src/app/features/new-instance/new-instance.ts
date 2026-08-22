import { ChangeDetectionStrategy, Component, computed, inject, output, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';

import { Api, OkDockError } from '../../core/api';
import { Store } from '../../core/state';
import { Provider, SpecRequest } from '../../core/models';
import { I18n } from '../../core/i18n/i18n';
import { ProviderForm } from '../../shared/provider-form';
import { GameIcon } from '../../shared/game-icon';
import { InfoDot } from '../../shared/info-dot';
import { ImageRef } from '../../shared/image-ref';

type Step = 1 | 2;

@Component({
  selector: 'gd-new-instance',
  imports: [FormsModule, ProviderForm, GameIcon, InfoDot, ImageRef],
  templateUrl: './new-instance.html',
  styleUrl: './new-instance.css',
  changeDetection: ChangeDetectionStrategy.OnPush,
  host: { '(document:keydown.escape)': 'close.emit()' },
})
export class NewInstance {
  private readonly api = inject(Api);
  readonly store = inject(Store);

  private readonly i18n = inject(I18n);
  readonly t = this.i18n.t;
  readonly problem = this.i18n.problem;

  readonly close = output<void>();
  readonly created = output<string>();

  readonly step = signal<Step>(1);
  readonly provider = signal<Provider | null>(null);
  readonly name = signal('');
  readonly image = signal('');
  readonly memoryLimit = signal('');
  readonly values = signal<Record<string, string>>({});
  readonly hostPorts = signal<Record<string, number>>({});
  readonly dnsDomain = signal('');
  readonly startAfterCreate = signal(true);

  readonly busy = signal(false);
  readonly error = signal<OkDockError | null>(null);

  readonly providers = computed(() => this.store.providers());

  readonly hasDnsToken = computed(() => !!this.store.dns()?.token);

  readonly dnsOptions = computed(() => {
    const dns = this.store.dns();
    if (!dns) return [];
    const taken = new Set(dns.links.map((l) => l.domain));
    return dns.domains.filter((d) => !taken.has(d.domain));
  });

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
    if (!/^[a-z0-9][a-z0-9_-]{1,38}$/.test(n)) return this.t('new.nameInvalid');
    if (this.store.instances().some((i) => i.name === n)) return this.t('new.nameTaken');
    return '';
  });

  readonly canAdvance = computed(() => {
    if (this.step() === 1) return !!this.provider();
    return !!this.name() && !this.nameError() && !!this.provider();
  });

  readonly budgetWarning = computed(() => {
    const sys = this.store.system();
    const p = this.provider();
    if (!sys || !p) return '';
    const want = parseMemory(this.memoryLimit() || p.defaultMemory);
    const free = sys.memoryBudget - sys.memoryCommitted;
    if (want > free) {
      return this.t('new.budgetWarning', {
        want: this.memoryLimit() || p.defaultMemory,
        free: (free / 1024 ** 3).toFixed(1),
      });
    }
    return '';
  });

  readonly hint = computed(() => {
    if (!this.provider()) return '';
    return this.t('new.hint', { name: this.name() || this.t('new.namePlaceholder') });
  });

  description(p: Provider): string {
    return this.i18n.maybe(`provider.${p.id}.desc`) ?? p.description;
  }

  portLabel(label: string | undefined): string {
    if (!label) return this.t('detail.portFallbackLabel');
    return this.i18n.maybe(`port.${label}`) ?? label;
  }

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
    this.dnsDomain.set('');
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
    this.submit();
  }

  back(): void {
    this.error.set(null);
    this.step.update((s) => (s > 1 ? ((s - 1) as Step) : s));
  }

  private submit(): void {
    this.busy.set(true);
    this.error.set(null);
    this.api.create({ ...this.request(), start: this.startAfterCreate() }).subscribe({
      next: () => this.linkThenClose(),
      error: (err: OkDockError) => {
        this.error.set(err);
        this.busy.set(false);
      },
    });
  }

  private linkThenClose(): void {
    const domain = this.dnsDomain();
    if (!domain) {
      this.busy.set(false);
      this.created.emit(this.name());
      return;
    }
    this.api.linkDns(this.name(), domain).subscribe({
      next: () => {
        this.busy.set(false);
        this.created.emit(this.name());
      },
      error: (err: OkDockError) => {
        this.busy.set(false);
        this.store.notify(
          this.t('new.dnsLinkFailed', { name: this.name(), domain, error: err.message }),
        );
        this.created.emit(this.name());
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
