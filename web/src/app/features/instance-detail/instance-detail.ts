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

import { Api, OkDockError } from '../../core/api';
import { Events } from '../../core/events';
import { Store } from '../../core/state';
import { Instance, STATE_KEY, SpecRequest, State } from '../../core/models';
import { I18n } from '../../core/i18n/i18n';
import { TemplateForm } from '../../shared/template-form';
import { GameIcon, templateColors } from '../../shared/game-icon';
import { InfoDot } from '../../shared/info-dot';
import { bytes } from '../../core/format';
import { copyText } from '../../core/clipboard';

type Tab = 'config' | 'console' | 'compose' | 'recursos';

const STATE_CHIP: Record<State, { bg: string; line: string; fg: string }> = {
  running: { bg: 'var(--ok-bg)', line: 'var(--ok-line)', fg: 'var(--ok)' },
  starting: { bg: 'var(--warn-bg)', line: 'var(--warn-line)', fg: 'var(--warn)' },
  provisioning: { bg: '#111b2b', line: '#2c3a4f', fg: 'var(--accent)' },
  updating: { bg: 'var(--busy-bg)', line: 'var(--busy-line)', fg: 'var(--busy)' },
  stopped: { bg: '#1a1d24', line: 'var(--line-strong)', fg: 'var(--fg-muted)' },
  error: { bg: 'var(--bad-bg)', line: 'var(--bad-line)', fg: 'var(--bad)' },
  archived: { bg: '#141519', line: 'var(--line)', fg: 'var(--fg-faint)' },
};

@Component({
  selector: 'ok-instance-detail',
  imports: [FormsModule, TemplateForm, GameIcon, InfoDot],
  templateUrl: './instance-detail.html',
  styleUrl: './instance-detail.css',
  changeDetection: ChangeDetectionStrategy.OnPush,
  host: { '(document:keydown.escape)': 'close.emit()' },
})
export class InstanceDetail {
  private readonly api = inject(Api);
  private readonly events = inject(Events);
  private readonly destroyRef = inject(DestroyRef);
  private readonly i18n = inject(I18n);
  readonly store = inject(Store);

  readonly t = this.i18n.t;
  readonly problem = this.i18n.problem;

  readonly name = input.required<string>();
  readonly close = output<void>();

  readonly tab = signal<Tab>('config');
  readonly values = signal<Record<string, string>>({});
  readonly memoryLimit = signal('');
  readonly cpus = signal(0);
  readonly hostPorts = signal<Record<string, number>>({});

  readonly recreate = signal<string[]>([]);
  readonly rawCompose = signal('');
  readonly logLines = signal<string[]>([]);
  readonly busy = signal(false);
  readonly error = signal<OkDockError | null>(null);

  readonly dnsDomain = signal('');
  readonly dnsBusy = signal(false);
  readonly dnsError = signal<string | null>(null);

  readonly instance = computed<Instance | undefined>(() =>
    this.store.instances().find((i) => i.name === this.name()),
  );

  readonly template = computed(() => {
    const i = this.instance();
    return i ? this.store.template(i.templateId) : undefined;
  });

  readonly chip = computed(() => {
    const state = this.instance()?.state ?? 'stopped';
    return { ...STATE_CHIP[state], label: this.t(STATE_KEY[state]) };
  });
  readonly colors = computed(() =>
    templateColors(this.instance()?.templateId ?? '', this.instance()?.category ?? 'other'),
  );

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

  // an outside container that cannot be edited says which of the reasons it is
  readonly readOnlyNote = computed(() => {
    const i = this.instance();
    if (!i) return '';
    const file = i.composeFile || '-';
    return (
      this.i18n.maybe(`detail.readOnly.${i.readOnly}`, { file }) ??
      this.t('detail.externalNote', { project: i.project || '-', dir: i.dir || '-' })
    );
  });

  readonly statsLine = computed(() => {
    const s = this.instance()?.stats;
    if (!s) return '';
    return this.t('detail.stats', {
      cpu: s.cpuPercent.toFixed(0),
      used: bytes(s.memoryBytes),
      total: bytes(s.memoryLimit),
    });
  });

  readonly createdLabel = computed(() => {
    const i = this.instance();
    return i ? this.t('detail.createdAt', { when: this.i18n.since(i.createdAt) }) : '';
  });

  portLabel(label: string | undefined): string {
    if (!label) return this.t('detail.portFallbackLabel');
    return this.i18n.maybe(`port.${label}`) ?? label;
  }

  readonly dnsSuffix = computed(() => this.store.dns()?.suffix ?? '.duckdns.org');
  readonly hasToken = computed(() => !!this.store.dns()?.token);

  readonly mainPort = computed(() => (this.instance()?.ports ?? [])[0]?.host ?? 0);

  readonly dnsAddress = computed(() => {
    const dns = this.instance()?.dns;
    if (!dns) return '';
    const port = this.mainPort();
    return port ? `${dns.hostname}:${port}` : dns.hostname;
  });

  readonly dnsSyncLabel = computed(() => {
    const at = this.instance()?.dns?.lastSync;
    return at ? this.i18n.since(at) : this.t('detail.neverChecked');
  });

  private loadedFor = '';

  constructor() {
    effect(() => {
      const i = this.instance();
      if (!i || this.loadedFor === i.name) return;
      this.loadedFor = i.name;
      if (this.tab() === 'config' && !this.template()) this.select('recursos');
      this.values.set({ ...i.env });
      this.memoryLimit.set(i.memoryLimit);
      this.cpus.set(i.cpus);
      this.hostPorts.set(
        Object.fromEntries((i.ports ?? []).map((p) => [`${p.container}/${p.protocol}`, p.host])),
      );
      this.error.set(null);
      this.dnsDomain.set(i.dns?.domain ?? '');
      this.dnsError.set(null);
      this.refreshRecreate();
    });
  }

  select(tab: Tab): void {
    this.tab.set(tab);
    if (tab === 'compose') {
      this.loadCompose();
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

  private loadCompose(): void {
    this.api.compose(this.name()).subscribe({
      next: (raw) => this.rawCompose.set(raw),
      error: () => {},
    });
  }

  refreshRecreate(): void {
    const req = this.request();
    if (!req) return;
    this.api.previewCompose(req).subscribe({
      next: (res) => this.recreate.set(res.recreate ?? []),
      error: () => {},
    });
  }

  setPort(container: number, protocol: string, raw: string): void {
    const n = Number(raw);
    if (!Number.isFinite(n)) return;
    this.hostPorts.update((cur) => ({ ...cur, [`${container}/${protocol}`]: n }));
    this.refreshRecreate();
  }

  save(): void {
    const req = this.request();
    if (!req) return;
    this.busy.set(true);
    this.error.set(null);
    this.api.update(this.name(), req).subscribe({
      next: () => {
        this.busy.set(false);
        this.rawCompose.set('');
        this.store.reload();
      },
      error: (err: OkDockError) => {
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
      error: (err: OkDockError) => {
        this.error.set(err);
        this.busy.set(false);
      },
    });
  }

  linkDns(): void {
    const domain = this.dnsDomain().trim();
    if (!domain) return;
    this.dnsBusy.set(true);
    this.dnsError.set(null);
    this.api.linkDns(this.name(), domain).subscribe({
      next: () => {
        this.dnsBusy.set(false);
        this.store.reload();
      },
      error: (err: OkDockError) => {
        this.dnsError.set(err.message);
        this.dnsBusy.set(false);
      },
    });
  }

  unlinkDns(): void {
    this.dnsBusy.set(true);
    this.dnsError.set(null);
    this.api.unlinkDns(this.name()).subscribe({
      next: () => {
        this.dnsBusy.set(false);
        this.dnsDomain.set('');
        this.store.reload();
      },
      error: (err: OkDockError) => {
        this.dnsError.set(err.message);
        this.dnsBusy.set(false);
      },
    });
  }

  syncDns(): void {
    this.api.syncDns().subscribe({
      next: () => this.store.notify(this.t('detail.syncing')),
      error: () => {},
    });
  }

  copyAddress(): void {
    const text = this.dnsAddress();
    if (!text) return;
    copyText(text);
    this.store.notify(this.t('common.copied', { text }));
  }

  private request(): SpecRequest | null {
    const i = this.instance();
    if (!i) return null;
    // what the template has no field for was added by hand, and rides apart or validation refuses it
    const known = new Set((this.template()?.fields ?? []).map((f) => f.key));
    const values: Record<string, string> = {};
    const extraEnv: Record<string, string> = {};
    for (const [key, value] of Object.entries(this.values())) {
      (known.has(key) ? values : extraEnv)[key] = value;
    }
    return {
      name: i.name,
      templateId: i.templateId,
      image: i.image,
      values,
      extraEnv,
      ports: (i.ports ?? []).map((p) => ({
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
