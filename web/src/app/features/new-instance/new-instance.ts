import { ChangeDetectionStrategy, Component, computed, inject, output, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';

import { Api, OkDockError } from '../../core/api';
import { Store } from '../../core/state';
import { Category, SpecRequest, Template } from '../../core/models';
import { I18n } from '../../core/i18n/i18n';
import { TemplateForm } from '../../shared/template-form';
import { GameIcon } from '../../shared/game-icon';
import { InfoDot } from '../../shared/info-dot';
import { ImageSearch } from '../../shared/image-search';
import { DirPicker, GhostDir } from '../../shared/dir-picker';

type Step = 1 | 2;

@Component({
  selector: 'ok-new-instance',
  imports: [FormsModule, TemplateForm, GameIcon, InfoDot, ImageSearch, DirPicker],
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

  readonly close = output<void>();
  readonly created = output<string>();

  readonly step = signal<Step>(1);
  readonly template = signal<Template | null>(null);
  readonly name = signal('');
  readonly image = signal('');
  readonly memoryLimit = signal('');
  readonly values = signal<Record<string, string>>({});
  readonly hostPorts = signal<Record<string, number>>({});
  readonly hostDirs = signal<Record<string, string>>({});
  readonly picking = signal<string | null>(null);
  readonly startAfterCreate = signal(true);

  readonly busy = signal(false);
  readonly error = signal<OkDockError | null>(null);

  readonly groups = computed(() => this.store.byCategory());

  // the host side opens on the port the template declared, the api is what refuses a taken one
  readonly ports = computed(() => {
    const p = this.template();
    if (!p) return [];
    const chosen = this.hostPorts();
    return (p.ports ?? []).map((port) => ({
      ...port,
      host: chosen[this.portKey(port.container, port.protocol)] ?? port.container,
    }));
  });

  // mirrors manager.mountsFor: a folder named after the last piece of the path, next to the compose file
  readonly volumes = computed(() => {
    const p = this.template();
    if (!p) return [];
    const chosen = this.hostDirs();
    const taken = new Set<string>();
    return (p.volumes ?? []).map((volume) => {
      let host = hostDirFor(volume.container);
      if (taken.has(host)) host = './' + volume.container.replace(/^\/+|\/+$/g, '').replace(/\//g, '-');
      taken.add(host);
      return { ...volume, host: chosen[volume.container] ?? host };
    });
  });

  readonly nameError = computed(() => {
    const n = this.name();
    if (!n) return '';
    if (!/^[a-z0-9][a-z0-9_-]{1,38}$/.test(n)) return this.t('new.nameInvalid');
    if (this.store.instances().some((i) => i.name === n)) return this.t('new.nameTaken');
    return '';
  });

  readonly canAdvance = computed(() => {
    if (this.step() === 1) return !!this.template();
    return !!this.name() && !this.nameError() && !!this.template();
  });

  readonly budgetWarning = computed(() => {
    const sys = this.store.system();
    const p = this.template();
    if (!sys || !p) return '';
    const want = parseMemory(this.memoryLimit() || p.defaultMemory);
    const free = sys.memoryBudget - sys.memoryCommitted;
    if (want > free) {
      return this.t('new.budgetWarning', {
        want: this.memoryLimit() || p.defaultMemory,
        free: (free / 1024 ** 3).toFixed(1),
      });
    }
    return '';
  });

  readonly hint = computed(() => {
    if (!this.template()) return '';
    return this.t('new.hint', { name: this.name() || this.t('new.namePlaceholder') });
  });

  categoryName(category: Category): string {
    return this.i18n.category(category);
  }

  portKey(container: number, protocol: string): string {
    return `${container}/${protocol}`;
  }

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

  pickDir(container: string): void {
    this.picking.set(container);
  }

  dirPicked(path: string): void {
    const container = this.picking();
    if (!container) return;
    const dir = this.instanceDir();
    const inside = dir && (path === dir || path.startsWith(dir + '/'));
    this.setVolume(container, inside ? '.' + path.slice(dir.length) : path);
    this.picking.set(null);
  }

  setVolume(container: string, host: string): void {
    this.hostDirs.update((cur) => ({ ...cur, [container]: host }));
  }

  pick(p: Template): void {
    this.template.set(p);
    this.image.set(p.image);
    this.memoryLimit.set(p.defaultMemory);
    const defaults: Record<string, string> = {};
    for (const f of p.fields ?? []) {
      if (f.default) defaults[f.key] = f.default;
    }
    this.values.set(defaults);
    this.hostPorts.set({});
    this.hostDirs.set({});
    this.error.set(null);
  }

  setPort(container: number, protocol: string, raw: string): void {
    const n = Number(raw);
    if (!Number.isFinite(n)) return;
    this.hostPorts.update((cur) => ({ ...cur, [this.portKey(container, protocol)]: n }));
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
    this.busy.set(true);
    this.error.set(null);
    this.api.create({ ...this.request(), start: this.startAfterCreate() }).subscribe({
      next: () => {
        this.busy.set(false);
        this.created.emit(this.name());
      },
      error: (err: OkDockError) => {
        this.error.set(err);
        this.busy.set(false);
      },
    });
  }

  private request(): SpecRequest {
    const p = this.template()!;
    return {
      name: this.name(),
      templateId: p.id,
      image: this.image() || undefined,
      values: this.values(),
      ports: this.ports().map((port) => ({
        host: port.host,
        container: port.container,
        protocol: port.protocol,
        label: port.label,
      })),
      mounts: this.volumes().map((volume) => ({ host: volume.host, container: volume.container })),
      memoryLimit: this.memoryLimit() || undefined,
    };
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
