import {
  ChangeDetectionStrategy,
  Component,
  computed,
  effect,
  inject,
  input,
  output,
  signal,
  untracked,
} from '@angular/core';
import { FormsModule } from '@angular/forms';

import { Store } from '../core/state';
import { ApiProblem, Mount, PortBinding, Template } from '../core/models';
import { I18n } from '../core/i18n/i18n';
import { TemplateForm } from './template-form';
import { InfoDot } from './info-dot';
import { ImageSearch } from './image-search';
import { GhostDir } from './dir-picker';
import { PickDir } from './pick-dir';
import { Select } from './select';

// a line of the ports table, fixed when the template is what declared that port
interface PortRow {
  id: number;
  container: string;
  protocol: 'tcp' | 'udp';
  host: string;
  label: string;
  fixed: boolean;
  autoHost: boolean;
}

interface VolumeRow {
  id: number;
  container: string;
  host: string;
  fixed: boolean;
  autoHost: boolean;
}

interface FieldRow {
  id: number;
  key: string;
  value: string;
}

// what an instance that already exists puts in the form
export interface InstanceFormSeed {
  name: string;
  image: string;
  memoryLimit: string;
  cpus: number;
  env: Record<string, string>;
  ports: PortBinding[];
  mounts: Mount[];
}

export interface InstanceFormValue {
  name: string;
  image: string;
  memoryLimit: string;
  cpus: number;
  values: Record<string, string>;
  extraEnv: Record<string, string>;
  ports: PortBinding[];
  mounts: Mount[];
}

@Component({
  selector: 'ok-instance-form',
  imports: [FormsModule, TemplateForm, InfoDot, ImageSearch, PickDir, Select],
  templateUrl: './instance-form.html',
  styleUrl: './instance-form.css',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class InstanceForm {
  readonly store = inject(Store);
  private readonly i18n = inject(I18n);
  readonly t = this.i18n.t;

  readonly template = input.required<Template>();
  // null fills the form from the template, an instance fills it from what is on disk
  readonly seed = input<InstanceFormSeed | null>(null);
  readonly mode = input<'create' | 'edit'>('create');
  readonly lockName = input(false);
  readonly problems = input<ApiProblem[]>([]);
  // what this instance already takes out of the RAM budget, empty while it does not exist yet
  readonly committed = input('');

  readonly changed = output<void>();

  readonly name = signal('');
  readonly image = signal('');
  readonly memoryLimit = signal('');
  readonly cpus = signal(0);
  readonly values = signal<Record<string, string>>({});
  readonly ports = signal<PortRow[]>([]);
  readonly volumes = signal<VolumeRow[]>([]);
  readonly fields = signal<FieldRow[]>([]);

  readonly protocols = [
    { value: 'tcp', label: 'tcp' },
    { value: 'udp', label: 'udp' },
  ];

  private nextId = 1;
  private seeded: unknown = null;

  constructor() {
    effect(() => {
      const template = this.template();
      const seed = this.seed();
      const key = seed ?? template.id;
      if (this.seeded === key) return;
      this.seeded = key;
      untracked(() => (seed ? this.fromInstance(template, seed) : this.fromTemplate(template)));
    });

    // every parent that saves wants to know, and reading the value is what says something moved
    effect(() => {
      this.value();
      this.changed.emit();
    });
  }

  private fromTemplate(template: Template): void {
    this.name.set('');
    this.image.set(template.image);
    this.memoryLimit.set(template.defaultMemory);
    this.cpus.set(template.defaultCpus);

    const defaults: Record<string, string> = {};
    for (const field of template.fields ?? []) {
      if (field.default) defaults[field.key] = field.default;
    }
    this.values.set(defaults);

    // the host side opens on the port the template declared, the api is what refuses a taken one
    this.ports.set(
      (template.ports ?? []).map((port) => ({
        id: this.nextId++,
        container: String(port.container),
        protocol: port.protocol,
        host: String(port.container),
        label: port.label,
        fixed: true,
        autoHost: false,
      })),
    );

    // mirrors manager.mountsFor: a folder named after the last piece of the path, next to the compose
    const taken = new Set<string>();
    this.volumes.set(
      (template.volumes ?? []).map((volume) => {
        let host = hostDirFor(volume.container);
        if (taken.has(host)) host = './' + volume.container.replace(/^\/+|\/+$/g, '').replace(/\//g, '-');
        taken.add(host);
        return { id: this.nextId++, container: volume.container, host, fixed: true, autoHost: false };
      }),
    );
    this.fields.set([]);
  }

  private fromInstance(template: Template, seed: InstanceFormSeed): void {
    this.name.set(seed.name);
    this.image.set(seed.image);
    this.memoryLimit.set(seed.memoryLimit);
    this.cpus.set(seed.cpus);

    // a variable the template has no field for was typed by hand, and it goes back to the same place
    const known = new Set((template.fields ?? []).map((field) => field.key));
    const values: Record<string, string> = {};
    const fields: FieldRow[] = [];
    for (const key of Object.keys(seed.env).sort()) {
      if (known.has(key)) values[key] = seed.env[key];
      else fields.push({ id: this.nextId++, key, value: seed.env[key] });
    }
    this.values.set(values);
    this.fields.set(fields);

    const declaredPorts = new Set(
      (template.ports ?? []).map((port) => `${port.container}/${port.protocol}`),
    );
    this.ports.set(
      (seed.ports ?? []).map((port) => ({
        id: this.nextId++,
        container: String(port.container),
        protocol: port.protocol,
        host: String(port.host),
        label: port.label ?? '',
        fixed: declaredPorts.has(`${port.container}/${port.protocol}`),
        autoHost: false,
      })),
    );

    const declaredVolumes = new Set((template.volumes ?? []).map((volume) => volume.container));
    this.volumes.set(
      (seed.mounts ?? []).map((mount) => ({
        id: this.nextId++,
        container: mount.container,
        host: mount.host,
        fixed: declaredVolumes.has(mount.container),
        autoHost: false,
      })),
    );
  }

  readonly editing = computed(() => this.mode() === 'edit');

  readonly nameError = computed(() => {
    const name = this.name().trim();
    if (!name) return '';
    if (!/^[a-z0-9][a-z0-9_-]{1,38}$/.test(name)) return this.t('new.nameInvalid');
    const mine = this.seed()?.name;
    if (name !== mine && this.store.instances().some((i) => i.name === name)) {
      return this.t('new.nameTaken');
    }
    return '';
  });

  readonly valid = computed(() => !!this.name().trim() && !this.nameError());

  readonly budgetWarning = computed(() => {
    const system = this.store.system();
    const template = this.template();
    if (!system) return '';
    const want = parseMemory(this.memoryLimit() || template.defaultMemory);
    const free = system.memoryBudget - system.memoryCommitted + parseMemory(this.committed());
    if (want <= free) return '';
    return this.t('new.budgetWarning', {
      want: this.memoryLimit() || template.defaultMemory,
      free: (free / 1024 ** 3).toFixed(1),
    });
  });

  readonly value = computed<InstanceFormValue>(() => ({
    name: this.name().trim(),
    image: this.image().trim(),
    memoryLimit: this.memoryLimit().trim(),
    cpus: this.cpus(),
    values: this.values(),
    extraEnv: Object.fromEntries(
      this.fields()
        .filter((field) => !!field.key.trim())
        .map((field) => [field.key.trim(), field.value]),
    ),
    ports: this.ports()
      .filter((port) => Number(port.container) > 0)
      .map((port) => ({
        host: Number(port.host) || Number(port.container),
        container: Number(port.container),
        protocol: port.protocol,
        label: port.label,
      })),
    mounts: this.volumes()
      .filter((volume) => !!volume.container.trim())
      .map((volume) => ({
        host: volume.host.trim() || hostDirFor(volume.container.trim()),
        container: volume.container.trim(),
      })),
  }));

  // the folder of the instance and the ones it is about to be given do not exist yet, the picker draws them anyway
  readonly ghostDirs = computed<GhostDir[]>(() => {
    const dir = this.instanceDir();
    if (!dir) return [];
    const ghosts: GhostDir[] = [{ path: dir }];
    for (const volume of this.volumes()) {
      ghosts.push({ path: this.absoluteDir(volume.host) });
    }
    return ghosts.filter((ghost) => !!ghost.path);
  });

  // before the name is typed the folder still has a place on the tree, under the name the field shows
  instanceDir(): string {
    const root = this.store.system()?.root;
    if (!root) return '';
    return `${root}/${this.name().trim() || this.t('new.namePlaceholder')}`;
  }

  // a relative folder hangs off the instance folder, which is where the compose file lands
  absoluteDir(host: string): string {
    if (host.startsWith('/')) return host;
    const dir = this.instanceDir();
    if (!dir) return '';
    return `${dir}/${host.replace(/^\.\//, '')}`;
  }

  // the picker opens on the root, since the folder of the instance itself is still a ghost
  pickerStart(): string {
    return this.store.system()?.root ?? '';
  }

  portLabel(label: string): string {
    if (!label) return '';
    return this.i18n.maybe(`port.${label}`) ?? label;
  }

  addPort(): void {
    this.ports.update((cur) => [
      ...cur,
      {
        id: this.nextId++,
        container: '',
        protocol: 'tcp',
        host: '',
        label: '',
        fixed: false,
        autoHost: true,
      },
    ]);
  }

  dropPort(id: number): void {
    this.ports.update((cur) => cur.filter((port) => port.id !== id));
  }

  // the host side follows what is typed on the container side until somebody types on it
  setPortContainer(id: number, raw: string): void {
    this.ports.update((cur) =>
      cur.map((port) =>
        port.id === id ? { ...port, container: raw, host: port.autoHost ? raw : port.host } : port,
      ),
    );
  }

  setPortHost(id: number, raw: string): void {
    this.ports.update((cur) =>
      cur.map((port) => (port.id === id ? { ...port, host: raw, autoHost: false } : port)),
    );
  }

  setPortProtocol(id: number, protocol: string): void {
    this.ports.update((cur) =>
      cur.map((port) => (port.id === id ? { ...port, protocol: protocol as 'tcp' | 'udp' } : port)),
    );
  }

  addVolume(): void {
    this.volumes.update((cur) => [
      ...cur,
      { id: this.nextId++, container: '', host: '', fixed: false, autoHost: true },
    ]);
  }

  dropVolume(id: number): void {
    this.volumes.update((cur) => cur.filter((volume) => volume.id !== id));
  }

  setVolumeContainer(id: number, raw: string): void {
    this.volumes.update((cur) =>
      cur.map((volume) =>
        volume.id === id
          ? { ...volume, container: raw, host: volume.autoHost ? hostDirFor(raw) : volume.host }
          : volume,
      ),
    );
  }

  setVolumeHost(id: number, raw: string): void {
    this.volumes.update((cur) =>
      cur.map((volume) => (volume.id === id ? { ...volume, host: raw, autoHost: false } : volume)),
    );
  }

  // a folder under the instance is written relative, so moving the instance takes it along
  volumePicked(id: number, path: string): void {
    const dir = this.instanceDir();
    const inside = dir && (path === dir || path.startsWith(dir + '/'));
    this.setVolumeHost(id, inside ? '.' + path.slice(dir.length) : path);
  }

  addField(): void {
    this.fields.update((cur) => [...cur, { id: this.nextId++, key: '', value: '' }]);
  }

  dropField(id: number): void {
    this.fields.update((cur) => cur.filter((field) => field.id !== id));
  }

  setFieldKey(id: number, raw: string): void {
    this.fields.update((cur) =>
      cur.map((field) => (field.id === id ? { ...field, key: raw } : field)),
    );
  }

  setFieldValue(id: number, raw: string): void {
    this.fields.update((cur) =>
      cur.map((field) => (field.id === id ? { ...field, value: raw } : field)),
    );
  }

  // a problem the api reported for a variable that has no field on the form
  fieldError(key: string): string {
    const problem = this.problems().find((p) => p.field === key);
    if (!problem) return '';
    return this.i18n.maybe(`problem.${problem.code}`, problem.params) ?? problem.code;
  }
}

function hostDirFor(dir: string): string {
  const name = dir.replace(/\/+$/, '').split('/').pop();
  if (!name || name === '.') return './data';
  return './' + name;
}

function parseMemory(s: string): number {
  const m = /^(\d+(?:\.\d+)?)\s*([gmk]?)b?$/i.exec(s.trim());
  if (!m) return 0;
  const mult = { g: 1024 ** 3, m: 1024 ** 2, k: 1024, '': 1 }[m[2].toLowerCase()] ?? 1;
  return Number(m[1]) * mult;
}
