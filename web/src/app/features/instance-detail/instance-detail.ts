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
  viewChild,
} from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { FormsModule } from '@angular/forms';

import { Api, OkDockError } from '../../core/api';
import { Events } from '../../core/events';
import { Store } from '../../core/state';
import { Instance, STATE_KEY, SpecRequest, State } from '../../core/models';
import { I18n } from '../../core/i18n/i18n';
import { InstanceForm, InstanceFormSeed } from '../../shared/instance-form';
import { TemplateIcon, templateColors } from '../../shared/template-icon';
import { InfoDot } from '../../shared/info-dot';
import { bytes } from '../../core/format';
import { CopyButton } from '../../shared/copy-button';

type Tab = 'config' | 'console' | 'compose';

const STATE_CHIP: Record<State, { bg: string; line: string; fg: string }> = {
  running: { bg: 'var(--ok-bg)', line: 'var(--ok-line)', fg: 'var(--ok)' },
  starting: { bg: 'var(--warn-bg)', line: 'var(--warn-line)', fg: 'var(--warn)' },
  provisioning: { bg: 'var(--accent-bg)', line: 'var(--accent-line)', fg: 'var(--accent)' },
  updating: { bg: 'var(--busy-bg)', line: 'var(--busy-line)', fg: 'var(--busy)' },
  stopped: { bg: 'var(--bg-toggle)', line: 'var(--line-strong)', fg: 'var(--fg-muted)' },
  error: { bg: 'var(--bad-bg)', line: 'var(--bad-line)', fg: 'var(--bad)' },
};

@Component({
  selector: 'ok-instance-detail',
  imports: [FormsModule, InstanceForm, TemplateIcon, InfoDot, CopyButton],
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
  readonly errorText = this.i18n.errorText;

  readonly name = input.required<string>();
  readonly close = output<void>();
  readonly renamed = output<string>();

  readonly form = viewChild(InstanceForm);

  readonly tab = signal<Tab>('config');
  readonly seed = signal<InstanceFormSeed | null>(null);

  readonly recreate = signal<string[]>([]);
  readonly rawCompose = signal('');
  readonly logLines = signal<string[]>([]);
  readonly busy = signal(false);
  readonly error = signal<OkDockError | null>(null);


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

  // the RAM this instance already holds is free again for the limit the form is typing
  readonly committed = computed(() => (this.isUp() ? (this.instance()?.memoryLimit ?? '') : ''));

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

  private loadedFor = '';
  private previewTimer = 0;

  constructor() {
    this.destroyRef.onDestroy(() => clearTimeout(this.previewTimer));

    effect(() => {
      const i = this.instance();
      if (!i || this.loadedFor === i.name) return;
      this.loadedFor = i.name;
      this.seed.set({
        name: i.name,
        image: i.image,
        memoryLimit: i.memoryLimit,
        cpus: i.cpus,
        env: { ...i.env },
        ports: i.ports ?? [],
        mounts: i.mounts ?? [],
      });
      this.error.set(null);
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

  // every keystroke of the form asks what would be recreated, and only the last one is worth asking
  refreshRecreate(): void {
    clearTimeout(this.previewTimer);
    this.previewTimer = setTimeout(() => this.preview(), 250);
  }

  private preview(): void {
    const req = this.request();
    if (!req) return;
    this.api.previewCompose(req, this.name()).subscribe({
      next: (res) => this.recreate.set(res.recreate ?? []),
      error: () => {},
    });
  }

  save(): void {
    const req = this.request();
    if (!req) return;
    this.busy.set(true);
    this.error.set(null);
    this.api.update(this.name(), req).subscribe({
      next: (spec) => {
        this.busy.set(false);
        this.rawCompose.set('');
        // the folder is still moving, and the screen follows the new name once the board has it
        if (spec.name && spec.name !== this.name()) {
          this.renamed.emit(spec.name);
        }
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

  // stopping the panel from itself leaves nothing to start it again, so the button asks twice
  readonly stopArmed = signal(false);
  private disarm?: ReturnType<typeof setTimeout>;

  stop(): void {
    if (this.instance()?.self && !this.stopArmed()) {
      this.stopArmed.set(true);
      clearTimeout(this.disarm);
      this.disarm = setTimeout(() => this.stopArmed.set(false), 4000);
      return;
    }
    clearTimeout(this.disarm);
    this.stopArmed.set(false);
    this.run(this.api.stop(this.name()));
  }

  restart(): void {
    this.run(this.api.restart(this.name()));
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

  private request(): SpecRequest | null {
    const i = this.instance();
    const value = this.form()?.value();
    if (!i || !value) return null;
    // no template behind a container from outside, and what it sends is merged into the file it has
    return {
      name: value.name,
      templateId: i.templateId,
      image: value.image,
      values: i.external ? { ...value.values, ...value.extraEnv } : value.values,
      extraEnv: i.external ? {} : value.extraEnv,
      ports: value.ports,
      mounts: value.mounts,
      memoryLimit: value.memoryLimit,
      cpus: value.cpus,
      restart: i.restart,
    };
  }
}
