import { ChangeDetectionStrategy, Component, computed, input, model, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';

import { Provider, ProviderField } from '../core/models';

@Component({
  selector: 'gd-provider-form',
  imports: [FormsModule],
  templateUrl: './provider-form.html',
  styleUrl: './provider-form.css',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ProviderForm {
  readonly provider = input.required<Provider>();
  readonly values = model.required<Record<string, string>>();
  readonly problems = input<string[]>([]);

  readonly showAdvanced = signal(false);

  readonly basic = computed(() => (this.provider().fields ?? []).filter((f) => !f.advanced));
  readonly advanced = computed(() => (this.provider().fields ?? []).filter((f) => f.advanced));

  readonly problemFor = computed(() => {
    const map = new Map<string, string>();
    for (const p of this.problems()) {
      const [key, ...rest] = p.split(':');
      if (rest.length) map.set(key.trim(), rest.join(':').trim());
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
}
