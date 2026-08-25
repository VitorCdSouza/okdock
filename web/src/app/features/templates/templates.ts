import { ChangeDetectionStrategy, Component, computed, inject, output, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';

import { Api, OkDockError } from '../../core/api';
import { Store } from '../../core/state';
import {
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
import { ImageSearch } from '../../shared/image-search';
import { Select } from '../../shared/select';

const FIELD_TYPES: FieldType[] = ['text', 'password', 'int', 'float', 'bool', 'enum'];

const CUSTOM_ID = 'custom';

type SuggestPart = 'ports' | 'volumes' | 'both';

// the API only takes a category matching ^[a-z0-9][a-z0-9-]{1,31}$
function slugify(raw: string): string {
  return raw
    .normalize('NFD')
    .replace(/[\u0300-\u036f]/g, '')
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .slice(0, 32)
    .replace(/^-+|-+$/g, '');
}

function blank(): Template {
  return {
    id: '',
    name: '',
    category: 'other',
    short: '',
    image: '',
    ports: [],
    volumes: [{ container: '/data' }],
    defaultMemory: '2g',
    minMemory: '512m',
    defaultCpus: 2,
    stopGraceSeconds: 30,
    fields: [],
  };
}

@Component({
  selector: 'ok-templates',
  imports: [FormsModule, GameIcon, InfoDot, ImageSearch, Select],
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
  readonly fieldTypeOptions = FIELD_TYPES.map((type) => ({ value: type, label: type }));
  readonly protocolOptions = [
    { value: 'tcp', label: 'tcp' },
    { value: 'udp', label: 'udp' },
  ];

  // a category typed here only reaches the API with the first template saved in it
  private readonly invented = signal<Category[]>([]);
  private readonly picked = signal<Category | null>(null);

  readonly categories = computed(() => {
    const known = this.store.categories();
    return [...known, ...this.invented().filter((c) => !known.includes(c))];
  });

  readonly categoryOptions = computed(() =>
    this.categories().map((c) => ({ value: c, label: this.categoryName(c) })),
  );

  readonly active = computed<Category>(() => this.picked() ?? this.categories()[0] ?? 'games');

  // the custom image is a hole to fill in the new instance, there is nothing to register here
  private readonly listed = computed(() =>
    this.store.templates().filter((t) => t.id !== CUSTOM_ID),
  );

  readonly counts = computed(() => {
    const counts = new Map<Category, number>();
    for (const t of this.listed()) {
      counts.set(t.category, (counts.get(t.category) ?? 0) + 1);
    }
    return counts;
  });

  readonly shown = computed(() => this.listed().filter((t) => t.category === this.active()));

  readonly adding = signal(false);
  readonly categoryDraft = signal('');

  readonly draft = signal<Template | null>(null);
  readonly editingId = signal('');
  readonly busy = signal(false);
  readonly error = signal<OkDockError | null>(null);
  readonly saved = signal(false);

  readonly isNew = computed(() => this.editingId() === '');

  categoryName(category: Category): string {
    return this.i18n.category(category);
  }

  count(category: Category): number {
    return this.counts().get(category) ?? 0;
  }

  select(category: Category): void {
    this.picked.set(category);
    this.adding.set(false);
  }

  startCategory(): void {
    this.adding.set(true);
    this.categoryDraft.set('');
  }

  cancelCategory(): void {
    this.adding.set(false);
  }

  confirmCategory(): void {
    const slug = slugify(this.categoryDraft());
    if (slug.length < 2) return;
    if (!this.categories().includes(slug)) {
      this.invented.update((list) => [...list, slug]);
    }
    this.adding.set(false);
    this.picked.set(slug);
    this.create();
  }

  edit(t: Template): void {
    this.picked.set(t.category);
    this.draft.set(structuredClone(t));
    this.editingId.set(t.id);
    this.suggestedFor = t.image;
    this.error.set(null);
    this.saved.set(false);
  }

  create(): void {
    this.draft.set({ ...blank(), category: this.active() });
    this.editingId.set('');
    this.suggestedFor = '';
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

  // which section is waiting on the image, so only the pressed button says it is loading
  readonly suggesting = signal<SuggestPart | null>(null);

  // the reference the form was filled from, so opening the same list again does not ask twice
  private suggestedFor = '';

  // the image knows its ports and volumes, a running container knows the rest, and the form is kept
  suggest(part: SuggestPart = 'both'): void {
    const draft = this.draft();
    if (!draft?.image || this.suggesting()) return;
    this.suggesting.set(part);
    this.suggestedFor = draft.image;
    this.api.suggestFromImage(draft.image).subscribe({
      next: (found) => {
        this.suggesting.set(null);
        const cur = this.draft();
        if (!cur) return;
        const ports = [...(cur.ports ?? [])];
        if (part !== 'volumes') {
          for (const p of found.ports) {
            if (ports.some((o) => o.container === p.container && o.protocol === p.protocol)) continue;
            ports.push(p);
          }
        }
        const volumes = [...(cur.volumes ?? [])];
        if (part !== 'ports') {
          for (const v of found.volumes) {
            if (volumes.some((o) => o.container === v.container)) continue;
            volumes.push(v);
          }
        }
        this.patch({ ports, volumes });
      },
      error: () => this.suggesting.set(null),
    });
  }

  // an image chosen from the list exists, so it fills the form and the buttons are a second look
  imagePicked(image: string): void {
    if (image === this.suggestedFor) return;
    this.suggest();
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
    this.patch({ volumes: [...cur.volumes, { container: '/data' }] });
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
