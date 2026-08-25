import { ChangeDetectionStrategy, Component, computed, effect, inject, output, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';

import { Api, OkDockError } from '../../core/api';
import { Store } from '../../core/state';
import { MetricPrefs, Prefs } from '../../core/prefs';
import { I18n } from '../../core/i18n/i18n';
import { Select } from '../../shared/select';
import { MessageKey } from '../../core/i18n/messages.pt';
import { InfoDot } from '../../shared/info-dot';

@Component({
  selector: 'ok-settings',
  imports: [FormsModule, InfoDot, Select],
  templateUrl: './settings.html',
  styleUrl: './settings.css',
  changeDetection: ChangeDetectionStrategy.OnPush,
  host: { '(document:keydown.escape)': 'close.emit()' },
})
export class Settings {
  private readonly api = inject(Api);
  readonly store = inject(Store);
  readonly prefs = inject(Prefs);
  readonly i18n = inject(I18n);

  readonly t = this.i18n.t;

  readonly languageOptions = computed(() => [
    { value: 'auto', label: this.t('settings.languageAuto') },
    { value: 'pt', label: 'Português' },
    { value: 'en', label: 'English' },
  ]);

  readonly close = output<void>();

  readonly metricOptions: { key: keyof MetricPrefs; label: MessageKey }[] = [
    { key: 'cpu', label: 'settings.metric.cpu' },
    { key: 'memory', label: 'settings.metric.memory' },
    { key: 'disk', label: 'settings.metric.disk' },
    { key: 'budget', label: 'settings.metric.budget' },
  ];

  readonly system = computed(() => this.store.system());
  readonly links = computed(() => this.store.dns()?.links ?? []);
  readonly domains = computed(() => this.store.dns()?.domains ?? []);
  readonly hasToken = computed(() => !!this.store.dns()?.token);
  readonly suffix = computed(() => this.store.dns()?.suffix ?? '.duckdns.org');

  readonly rootDraft = signal('');
  readonly rootBusy = signal(false);
  readonly rootError = signal<string | null>(null);
  readonly rootSaved = signal(false);

  readonly rootChanged = computed(() => {
    const draft = this.rootDraft().trim();
    return !!draft && draft !== this.system()?.root;
  });

  readonly templatesDraft = signal('');
  readonly templatesBusy = signal(false);
  readonly templatesError = signal<string | null>(null);
  readonly templatesSaved = signal(false);

  readonly templatesChanged = computed(() => {
    const draft = this.templatesDraft().trim();
    return !!draft && draft !== this.system()?.templatesRoot;
  });

  readonly tokenDraft = signal('');
  readonly tokenHidden = signal(false);
  readonly tokenBusy = signal(false);
  readonly tokenError = signal<string | null>(null);
  readonly tokenNote = signal<string | null>(null);

  readonly drafts = signal<string[]>([]);
  readonly busyDomain = signal<string | null>(null);
  readonly domainError = signal<string | null>(null);

  readonly dockerLabel = computed(() => {
    const s = this.system();
    if (!s) return '-';
    return s.dockerVersion
      ? this.t('settings.dockerVersion', { version: s.dockerVersion })
      : this.t('settings.dockerSilent');
  });

  readonly dockerDetail = computed(() => this.system()?.dockerError ?? '');

  constructor() {
    effect(() => {
      const root = this.system()?.root;
      if (root && !this.rootDraft()) this.rootDraft.set(root);
    });
    effect(() => {
      const dir = this.system()?.templatesRoot;
      if (dir && !this.templatesDraft()) this.templatesDraft.set(dir);
    });
    effect(() => {
      const token = this.store.dns()?.token;
      if (token && !this.tokenDraft()) {
        this.tokenDraft.set(token);
        this.tokenHidden.set(true);
      }
    });
  }

  saveTemplates(): void {
    const dir = this.templatesDraft().trim();
    if (!dir || this.templatesBusy()) return;
    this.templatesBusy.set(true);
    this.templatesError.set(null);
    this.templatesSaved.set(false);
    this.api.setTemplatesRoot(dir).subscribe({
      next: (info) => {
        this.store.system.set(info);
        this.templatesBusy.set(false);
        this.templatesSaved.set(true);
        this.store.reload();
      },
      error: (err: OkDockError) => {
        this.templatesError.set(err.message);
        this.templatesBusy.set(false);
      },
    });
  }

  saveRoot(): void {
    const root = this.rootDraft().trim();
    if (!root || this.rootBusy()) return;
    this.rootBusy.set(true);
    this.rootError.set(null);
    this.rootSaved.set(false);
    this.api.setRoot(root).subscribe({
      next: (info) => {
        this.store.system.set(info);
        this.rootBusy.set(false);
        this.rootSaved.set(true);
        this.store.reload();
      },
      error: (err: OkDockError) => {
        this.rootError.set(err.message);
        this.rootBusy.set(false);
      },
    });
  }

  onTokenInput(value: string): void {
    this.tokenDraft.set(value);
    this.tokenNote.set(null);
  }

  editToken(): void {
    this.tokenHidden.set(false);
    this.tokenNote.set(null);
  }

  saveToken(): void {
    const token = this.tokenDraft().trim();
    if (!token || this.tokenBusy()) return;
    this.tokenBusy.set(true);
    this.tokenError.set(null);
    this.tokenNote.set(null);
    this.api.saveDnsToken(token).subscribe({
      next: () => {
        this.tokenBusy.set(false);
        this.tokenHidden.set(true);
        this.store.reloadDns();
        if (this.domains().length) {
          this.api.syncDns().subscribe({ error: () => {} });
          this.tokenNote.set(this.t('settings.tokenSavedChecking'));
        } else {
          this.tokenNote.set(this.t('settings.tokenSavedPending'));
        }
      },
      error: (err: OkDockError) => {
        this.tokenError.set(err.message);
        this.tokenBusy.set(false);
      },
    });
  }

  draftKey(index: number): string {
    return `draft:${index}`;
  }

  instanceFor(domain: string): string {
    return this.links().find((l) => l.domain === domain)?.instance ?? '';
  }

  addDraft(): void {
    this.drafts.update((d) => [...d, '']);
    this.domainError.set(null);
  }

  setDraft(index: number, value: string): void {
    this.drafts.update((d) => d.map((cur, i) => (i === index ? value : cur)));
  }

  dropDraft(index: number): void {
    this.drafts.update((d) => d.filter((_, i) => i !== index));
    this.domainError.set(null);
  }

  saveDraft(index: number, value: string): void {
    const domain = value.trim();
    if (!domain || this.busyDomain()) return;
    this.busyDomain.set(this.draftKey(index));
    this.domainError.set(null);
    this.api.addDnsDomain(domain).subscribe({
      next: () => {
        this.busyDomain.set(null);
        this.dropDraft(index);
        this.store.reloadDns();
      },
      error: (err: OkDockError) => {
        this.domainError.set(err.message);
        this.busyDomain.set(null);
      },
    });
  }

  rename(current: string, value: string): void {
    const domain = value.trim();
    if (!domain || domain === current || this.busyDomain()) return;
    this.busyDomain.set(current);
    this.domainError.set(null);
    this.api.addDnsDomain(domain).subscribe({
      next: () => {
        this.api.removeDnsDomain(current).subscribe({
          next: () => {
            this.busyDomain.set(null);
            this.store.reloadDns();
          },
          error: () => {
            this.busyDomain.set(null);
            this.store.reloadDns();
          },
        });
      },
      error: (err: OkDockError) => {
        this.domainError.set(err.message);
        this.busyDomain.set(null);
        this.store.reloadDns();
      },
    });
  }

  removeDomain(domain: string): void {
    if (this.busyDomain()) return;
    this.busyDomain.set(domain);
    this.domainError.set(null);
    this.api.removeDnsDomain(domain).subscribe({
      next: () => {
        this.busyDomain.set(null);
        this.store.reloadDns();
      },
      error: (err: OkDockError) => {
        this.domainError.set(err.message);
        this.busyDomain.set(null);
      },
    });
  }

  sync(): void {
    this.api.syncDns().subscribe({
      next: () => this.store.notify(this.t('settings.syncing')),
      error: (err: OkDockError) => this.tokenError.set(err.message),
    });
  }
}
