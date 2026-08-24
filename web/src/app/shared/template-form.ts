import { ChangeDetectionStrategy, Component, computed, inject, input, model, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';

import { ApiProblem, Template, TemplateField } from '../core/models';
import { I18n } from '../core/i18n/i18n';
import { InfoDot } from './info-dot';
import { Select } from './select';

@Component({
  selector: 'ok-template-form',
  imports: [FormsModule, InfoDot, Select],
  templateUrl: './template-form.html',
  styleUrl: './template-form.css',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class TemplateForm {
  private readonly i18n = inject(I18n);
  readonly t = this.i18n.t;

  readonly template = input.required<Template>();
  readonly values = model.required<Record<string, string>>();
  readonly problems = input<ApiProblem[]>([]);

  readonly showAdvanced = signal(false);

  readonly basic = computed(() => (this.template().fields ?? []).filter((f) => !f.advanced));
  readonly advanced = computed(() => (this.template().fields ?? []).filter((f) => f.advanced));

  readonly problemFor = computed(() => {
    const map = new Map<string, string>();
    for (const p of this.problems()) {
      map.set(p.field, this.i18n.maybe(`problem.${p.code}`, p.params) ?? p.code);
    }
    return map;
  });

  value(field: TemplateField): string {
    return this.values()[field.key] ?? field.default ?? '';
  }

  set(field: TemplateField, raw: string | boolean): void {
    const v = typeof raw === 'boolean' ? String(raw) : raw;
    this.values.update((cur) => ({ ...cur, [field.key]: v }));
  }

  checked(field: TemplateField): boolean {
    return this.value(field) === 'true';
  }

  error(field: TemplateField): string | undefined {
    return this.problemFor().get(field.key);
  }

  fieldHelp(field: TemplateField): string {
    return this.i18n.maybe(`field.${this.template().id}.${field.key}.help`) ?? field.help ?? '';
  }
}
