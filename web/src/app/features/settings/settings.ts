import { ChangeDetectionStrategy, Component, computed, effect, inject, output, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { Observable, catchError, concat, of, tap } from 'rxjs';

import { Api, OkDockError } from '../../core/api';
import { Store } from '../../core/state';
import { MetricPrefs, Prefs } from '../../core/prefs';
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

  readonly metricDraft = signal<MetricPrefs | null>(null);
  readonly metrics = computed(() => this.metricDraft() ?? this.prefs.metrics());

  readonly languageDraft = signal<LocalePref | null>(null);
  readonly language = computed(() => this.languageDraft() ?? this.i18n.pref());

  readonly dirty = computed(
    () =>
      this.rootChanged() ||
      this.templatesChanged() ||
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

  onEscape(): void {
    this.close.emit();
  }

  // the picker already asked which folder, and the save button writes it
  pickFolder(which: 'root' | 'templates', path: string): void {
    if (which === 'root') {
      this.rootDraft.set(path);
      return;
    }
    this.templatesDraft.set(path);
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

    const folders = this.rootChanged() || this.templatesChanged();

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

    concat(...steps).subscribe({ complete: () => this.done(folders) });
  }

  private done(folders: boolean): void {
    this.prefs.setMetrics(this.metrics());
    this.i18n.setPref(this.language());
    this.metricDraft.set(null);
    this.languageDraft.set(null);
    this.busy.set(false);
    this.saved.set(true);

    if (folders) this.store.reload();
  }
}
