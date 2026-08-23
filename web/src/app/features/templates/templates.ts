import { ChangeDetectionStrategy, Component, computed, inject, output, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';

import { Api, OkDockError } from '../../core/api';
import { Store } from '../../core/state';
import {
  CATEGORY_KEY,
  Category,
  FieldType,
  Template,
  TemplateField,
  TemplatePort,
  TemplateVolume,
} from '../../core/models';
import { I18n } from '../../core/i18n/i18n';
import { GameIcon } from '../../shared/game-icon';
import { InfoDot } from '../../shared/info-dot';

const FIELD_TYPES: FieldType[] = ['text', 'password', 'int', 'float', 'bool', 'enum'];

function blank(): Template {
  return {
    id: '',
    name: '',
    category: 'other',
    short: '',
    image: '',
    description: '',
    ports: [],
    volumes: [{ host: './data', container: '/data', data: true }],
    defaultMemory: '2g',
    minMemory: '512m',
    defaultCpus: 2,
    stopGraceSeconds: 30,
    fields: [],
  };
}

@Component({
  selector: 'ok-templates',
  imports: [FormsModule, GameIcon, InfoDot],
  templateUrl: './templates.html',
  styleUrl: './templates.css',
  changeDetection: ChangeDetectionStrategy.OnPush,
  host: { '(document:keydown.escape)': 'close.emit()' },
})
export class Templates {
  private readonly api = inject(Api);
  private readonly i18n = inject(I18n);
  readonly store = inject(Store);

  readonly t = this.i18n.t;
  readonly problem = this.i18n.problem;

  readonly close = output<void>();

  readonly fieldTypes = FIELD_TYPES;
  readonly categories = computed(() => this.store.categories());
  readonly groups = computed(() => this.store.byCategory());

  readonly draft = signal<Template | null>(null);
  readonly editingId = signal('');
  readonly busy = signal(false);
  readonly error = signal<OkDockError | null>(null);
  readonly saved = signal(false);

  readonly isNew = computed(() => this.editingId() === '');

  readonly tagsText = computed(() => (this.draft()?.tags ?? []).join(', '));

  categoryKey(category: Category) {
    return CATEGORY_KEY[category];
  }

  edit(t: Template): void {
    this.draft.set(structuredClone(t));
    this.editingId.set(t.id);
    this.error.set(null);
    this.saved.set(false);
  }

  create(): void {
    this.draft.set(blank());
    this.editingId.set('');
    this.error.set(null);
    this.saved.set(false);
  }

  cancel(): void {
    this.draft.set(null);
    this.error.set(null);
  }

  patch(change: Partial<Template>): void {
    const cur = this.draft();
    if (!cur) return;
    this.draft.set({ ...cur, ...change });
  }

  setTags(raw: string): void {
    const tags = raw
      .split(',')
      .map((t) => t.trim())
      .filter(Boolean);
    this.patch({ tags: tags.length ? tags : undefined });
  }

  addPort(): void {
    const cur = this.draft();
    if (!cur) return;
    const port: TemplatePort = { container: 8080, protocol: 'tcp', defaultHost: 8080, label: 'web' };
    this.patch({ ports: [...(cur.ports ?? []), port] });
  }

  patchPort(index: number, change: Partial<TemplatePort>): void {
    const ports = [...(this.draft()?.ports ?? [])];
    ports[index] = { ...ports[index], ...change };
    this.patch({ ports });
  }

  removePort(index: number): void {
    const ports = [...(this.draft()?.ports ?? [])];
    ports.splice(index, 1);
    this.patch({ ports });
  }

  addVolume(): void {
    const cur = this.draft();
    if (!cur) return;
    this.patch({ volumes: [...cur.volumes, { host: './data', container: '/data' }] });
  }

  patchVolume(index: number, change: Partial<TemplateVolume>): void {
    const volumes = [...(this.draft()?.volumes ?? [])];
    volumes[index] = { ...volumes[index], ...change };
    this.patch({ volumes });
  }

  removeVolume(index: number): void {
    const volumes = [...(this.draft()?.volumes ?? [])];
    volumes.splice(index, 1);
    this.patch({ volumes });
  }

  addField(): void {
    const cur = this.draft();
    if (!cur) return;
    const field: TemplateField = { key: '', label: '', type: 'text' };
    this.patch({ fields: [...(cur.fields ?? []), field] });
  }

  patchField(index: number, change: Partial<TemplateField>): void {
    const fields = [...(this.draft()?.fields ?? [])];
    fields[index] = { ...fields[index], ...change };
    this.patch({ fields });
  }

  removeField(index: number): void {
    const fields = [...(this.draft()?.fields ?? [])];
    fields.splice(index, 1);
    this.patch({ fields });
  }

  optionsText(field: TemplateField): string {
    return (field.options ?? []).map((o) => (o.label === o.value ? o.value : `${o.value}=${o.label}`)).join(', ');
  }

  setOptions(index: number, raw: string): void {
    const options = raw
      .split(',')
      .map((part) => part.trim())
      .filter(Boolean)
      .map((part) => {
        const [value, label] = part.split('=');
        return { value: value.trim(), label: (label ?? value).trim() };
      });
    this.patchField(index, { options: options.length ? options : undefined });
  }

  save(): void {
    const draft = this.draft();
    if (!draft) return;
    this.busy.set(true);
    this.error.set(null);

    const call = this.isNew() ? this.api.createTemplate(draft) : this.api.saveTemplate(draft);
    call.subscribe({
      next: (saved) => {
        this.busy.set(false);
        this.saved.set(true);
        this.editingId.set(saved.id);
        this.draft.set(structuredClone(saved));
        this.store.reloadTemplates();
      },
      error: (err: OkDockError) => {
        this.error.set(err);
        this.busy.set(false);
      },
    });
  }

  remove(): void {
    const draft = this.draft();
    if (!draft || this.isNew()) return;
    this.busy.set(true);
    this.error.set(null);
    this.api.deleteTemplate(draft.id).subscribe({
      next: () => {
        this.busy.set(false);
        this.draft.set(null);
        this.store.reloadTemplates();
      },
      error: (err: OkDockError) => {
        this.error.set(err);
        this.busy.set(false);
      },
    });
  }
}
