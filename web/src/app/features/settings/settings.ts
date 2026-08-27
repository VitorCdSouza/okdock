import { ChangeDetectionStrategy, Component, computed, effect, inject, output, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { EMPTY, Observable, catchError, concat, defer, of, tap } from 'rxjs';

import { Api, OkDockError } from '../../core/api';
import { Store } from '../../core/state';
import { MetricPrefs, Prefs } from '../../core/prefs';
import { InstanceDNS } from '../../core/models';
import { I18n, LocalePref } from '../../core/i18n/i18n';
import { Select } from '../../shared/select';
import { MessageKey } from '../../core/i18n/messages.pt';
import { InfoDot } from '../../shared/info-dot';
import { PickDir } from '../../shared/pick-dir';

@Component({
  selector: 'ok-settings',
  imports: [FormsModule, PickDir, InfoDot, Select],
  templateUrl: './settings.html',
  styleUrl: './settings.css',
  changeDetection: ChangeDetectionStrategy.OnPush,
  host: { '(document:keydown.escape)': 'onEscape()' },
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

  readonly busy = signal(false);
  readonly saved = signal(false);

  readonly rootDraft = signal('');
  readonly rootError = signal<string | null>(null);

  readonly rootChanged = computed(() => {
    const draft = this.rootDraft().trim();
    return !!draft && draft !== this.system()?.root;
  });

  readonly templatesDraft = signal('');
  readonly templatesError = signal<string | null>(null);

  readonly templatesChanged = computed(() => {
    const draft = this.templatesDraft().trim();
    return !!draft && draft !== this.system()?.templatesRoot;
  });

  // the token is written from its own dialog, and the panel never shows the saved one back
  readonly tokenOpen = signal(false);
  readonly tokenDraft = signal('');
  readonly tokenBusy = signal(false);
  readonly tokenError = signal<string | null>(null);
  readonly tokenNote = signal<string | null>(null);

  // a name duckdns refused is what a wrong token looks like from here
  readonly refused = computed(() => this.domains().filter((d) => !!d.lastError));

  // a draft is null while nothing was touched, so the screen keeps following the server
  readonly nameDraft = signal<string[] | null>(null);
  readonly domainError = signal<string | null>(null);

  readonly serverNames = computed(() => this.domains().map((d) => d.domain));
  readonly names = computed(() => this.nameDraft() ?? this.serverNames());

  readonly toAdd = computed(() => {
    const known = this.serverNames();
    return this.names()
      .map((name) => name.trim())
      .filter((name) => !!name && !known.includes(name));
  });

  readonly toRemove = computed(() => {
    const kept = this.names().map((name) => name.trim());
    return this.serverNames().filter((name) => !kept.includes(name));
  });

  readonly metricDraft = signal<MetricPrefs | null>(null);
  readonly metrics = computed(() => this.metricDraft() ?? this.prefs.metrics());

  readonly languageDraft = signal<LocalePref | null>(null);
  readonly language = computed(() => this.languageDraft() ?? this.i18n.pref());

  readonly dirty = computed(
    () =>
      this.rootChanged() ||
      this.templatesChanged() ||
      !!this.toAdd().length ||
      !!this.toRemove().length ||
      this.metricOptions.some((m) => this.metrics()[m.key] !== this.prefs.metrics()[m.key]) ||
      this.language() !== this.i18n.pref(),
  );

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
  }

  // the picker already asked which folder, and the save button writes it
  pickFolder(which: 'root' | 'templates', path: string): void {
    if (which === 'root') {
      this.rootDraft.set(path);
      return;
    }
    this.templatesDraft.set(path);
  }

  onEscape(): void {
    if (this.tokenOpen()) {
      this.closeToken();
      return;
    }
    this.close.emit();
  }

  openToken(): void {
    this.tokenDraft.set('');
    this.tokenError.set(null);
    this.tokenNote.set(null);
    this.tokenOpen.set(true);
  }

  closeToken(): void {
    this.tokenOpen.set(false);
    this.tokenDraft.set('');
    this.tokenError.set(null);
  }

  saveToken(): void {
    const token = this.tokenDraft().trim();
    if (!token || this.tokenBusy()) return;
    this.tokenBusy.set(true);
    this.tokenError.set(null);
    this.tokenNote.set(null);
    this.api.saveDnsToken(token).subscribe({
      next: (status) => {
        this.store.dns.set(status);
        this.tokenBusy.set(false);
        this.tokenOpen.set(false);
        this.tokenDraft.set('');
        this.checkToken();
      },
      error: (err: OkDockError) => {
        this.tokenError.set(err.message);
        this.tokenBusy.set(false);
      },
    });
  }

  // duckdns only answers about a name that exists, so the names on the list are the check
  private checkToken(): void {
    if (!this.serverNames().length) {
      this.tokenNote.set(this.t('settings.tokenSavedPending'));
      return;
    }
    this.tokenNote.set(this.t('settings.tokenSavedChecking'));
    this.api.syncDns().subscribe({
      next: () => {
        this.tokenNote.set(null);
        this.store.reloadDns();
      },
      error: (err: OkDockError) => {
        this.tokenNote.set(null);
        this.tokenError.set(err.message);
      },
    });
  }

  instanceFor(domain: string): string {
    return this.links().find((l) => l.domain === domain)?.instance ?? '';
  }

  statusFor(domain: string): InstanceDNS | undefined {
    return this.domains().find((d) => d.domain === domain);
  }

  addName(): void {
    this.nameDraft.set([...this.names(), '']);
    this.domainError.set(null);
  }

  setName(index: number, value: string): void {
    this.nameDraft.set(this.names().map((name, i) => (i === index ? value : name)));
  }

  dropName(index: number): void {
    this.nameDraft.set(this.names().filter((_, i) => i !== index));
    this.domainError.set(null);
  }

  toggleMetric(key: keyof MetricPrefs): void {
    this.metricDraft.set({ ...this.metrics(), [key]: !this.metrics()[key] });
  }

  setLanguage(pref: LocalePref): void {
    this.languageDraft.set(pref);
  }

  save(): void {
    if (this.busy() || !this.dirty()) return;
    this.busy.set(true);
    this.saved.set(false);
    this.rootError.set(null);
    this.templatesError.set(null);
    this.domainError.set(null);

    const folders = this.rootChanged() || this.templatesChanged();
    const dns = !!this.toAdd().length || !!this.toRemove().length;
    let addOk = true;

    const steps: Observable<unknown>[] = [];

    if (this.rootChanged()) {
      steps.push(
        this.api.setRoot(this.rootDraft().trim()).pipe(
          tap((info) => this.store.system.set(info)),
          catchError((err: OkDockError) => {
            this.rootError.set(err.message);
            return of(null);
          }),
        ),
      );
    }

    if (this.templatesChanged()) {
      steps.push(
        this.api.setTemplatesRoot(this.templatesDraft().trim()).pipe(
          tap((info) => this.store.system.set(info)),
          catchError((err: OkDockError) => {
            this.templatesError.set(err.message);
            return of(null);
          }),
        ),
      );
    }

    for (const name of this.toAdd()) {
      steps.push(
        this.api.addDnsDomain(name).pipe(
          catchError((err: OkDockError) => {
            addOk = false;
            this.domainError.set(err.message);
            return of(null);
          }),
        ),
      );
    }

    // a refused name leaves the old one standing, or a typo would erase the address
    for (const name of this.toRemove()) {
      steps.push(
        defer(() =>
          addOk
            ? this.api.removeDnsDomain(name).pipe(
                catchError((err: OkDockError) => {
                  this.domainError.set(err.message);
                  return of(null);
                }),
              )
            : EMPTY,
        ),
      );
    }

    concat(...steps).subscribe({ complete: () => this.done(folders, dns) });
  }

  private done(folders: boolean, dns: boolean): void {
    this.prefs.setMetrics(this.metrics());
    this.i18n.setPref(this.language());
    this.metricDraft.set(null);
    this.languageDraft.set(null);
    this.nameDraft.set(null);
    this.busy.set(false);
    this.saved.set(true);

    if (folders) this.store.reload();
    if (dns) this.store.reloadDns();
  }

  sync(): void {
    this.api.syncDns().subscribe({
      next: () => this.store.notify(this.t('settings.syncing')),
      error: (err: OkDockError) => this.tokenError.set(err.message),
    });
  }
}
