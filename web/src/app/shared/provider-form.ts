import { ChangeDetectionStrategy, Component, computed, inject, input, model, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';

import { ApiProblem, Provider, ProviderField } from '../core/models';
import { I18n } from '../core/i18n/i18n';
import { InfoDot } from './info-dot';

@Component({
  selector: 'gd-provider-form',
  imports: [FormsModule, InfoDot],
  templateUrl: './provider-form.html',
  styleUrl: './provider-form.css',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ProviderForm {
  private readonly i18n = inject(I18n);
  readonly t = this.i18n.t;

  readonly provider = input.required<Provider>();
  readonly values = model.required<Record<string, string>>();
  readonly problems = input<ApiProblem[]>([]);

  readonly showAdvanced = signal(false);

  readonly basic = computed(() => (this.provider().fields ?? []).filter((f) => !f.advanced));
  readonly advanced = computed(() => (this.provider().fields ?? []).filter((f) => f.advanced));

  readonly problemFor = computed(() => {
    const map = new Map<string, string>();
    for (const p of this.problems()) {
      map.set(p.field, this.i18n.maybe(`problem.${p.code}`, p.params) ?? p.code);
    }
    return map;
  });

  value(field: ProviderField): string {
    return this.values()[field.key] ?? field.default ?? '';
  }

  set(field: ProviderField, raw: string | boolean): void {
    const v = typeof raw === 'boolean' ? String(raw) : raw;
    this.values.update((cur) => ({ ...cur, [field.key]: v }));
  }

  checked(field: ProviderField): boolean {
    return this.value(field) === 'true';
  }

  error(field: ProviderField): string | undefined {
    return this.problemFor().get(field.key);
  }

  // o texto do catalogo sobra quando o campo e de um jogo que a tela ainda nao traduziu
  fieldHelp(field: ProviderField): string {
    return this.i18n.maybe(`field.${this.provider().id}.${field.key}.help`) ?? field.help ?? '';
  }
}
