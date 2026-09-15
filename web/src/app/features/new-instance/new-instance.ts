import {
  ChangeDetectionStrategy,
  Component,
  computed,
  inject,
  output,
  signal,
  viewChild,
} from '@angular/core';

import { Api, OkDockError } from '../../core/api';
import { Store } from '../../core/state';
import { Category, SpecRequest, Template } from '../../core/models';
import { I18n } from '../../core/i18n/i18n';
import { InstanceForm } from '../../shared/instance-form';
import { TemplateIcon } from '../../shared/template-icon';
import { InfoDot } from '../../shared/info-dot';
import { CopyButton } from '../../shared/copy-button';

type Step = 1 | 2;

@Component({
  selector: 'ok-new-instance',
  imports: [InstanceForm, TemplateIcon, InfoDot, CopyButton],
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
  readonly errorText = this.i18n.errorText;

  readonly close = output<void>();
  readonly created = output<string>();

  readonly form = viewChild(InstanceForm);

  readonly step = signal<Step>(1);
  readonly template = signal<Template | null>(null);
  readonly startAfterCreate = signal(true);

  readonly busy = signal(false);
  readonly error = signal<OkDockError | null>(null);

  readonly groups = computed(() => this.store.byCategory());

  readonly canAdvance = computed(() => {
    if (this.step() === 1) return !!this.template();
    return !!this.form()?.valid();
  });

  readonly hint = computed(() => {
    if (!this.template()) return '';
    const name = this.form()?.value().name;
    return this.t('new.hint', { name: name || this.t('new.namePlaceholder') });
  });

  categoryName(category: Category): string {
    return this.i18n.category(category);
  }

  pick(template: Template): void {
    this.template.set(template);
    this.error.set(null);
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
    const request = this.request();
    if (!request) return;
    this.busy.set(true);
    this.error.set(null);
    this.api.create(request).subscribe({
      next: () => {
        this.busy.set(false);
        this.created.emit(request.name);
      },
      error: (err: OkDockError) => {
        this.error.set(err);
        this.busy.set(false);
      },
    });
  }

  private request(): SpecRequest | null {
    const template = this.template();
    const value = this.form()?.value();
    if (!template || !value) return null;
    return {
      name: value.name,
      templateId: template.id,
      image: value.image || undefined,
      values: value.values,
      extraEnv: value.extraEnv,
      ports: value.ports,
      mounts: value.mounts,
      memoryLimit: value.memoryLimit || undefined,
      start: this.startAfterCreate(),
    };
  }
}
