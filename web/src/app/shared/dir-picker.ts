import {
  ChangeDetectionStrategy,
  Component,
  OnInit,
  computed,
  inject,
  input,
  output,
  signal,
} from '@angular/core';
import { FormsModule } from '@angular/forms';

import { Api, OkDockError } from '../core/api';
import { DirEntry } from '../core/models';
import { I18n } from '../core/i18n/i18n';

// a folder the instance is going to be given, drawn before anybody created it
export interface GhostDir {
  path: string;
  label?: string;
}

export interface TreeNode {
  name: string;
  path: string;
  depth: number;
  ghost: boolean;
  open: boolean;
  loading: boolean;
  leaf: boolean;
}

@Component({
  selector: 'ok-dir-picker',
  imports: [FormsModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  host: { '(document:keydown.escape)': 'close.emit()' },
  template: `
    <div class="backdrop" (click)="close.emit()"></div>

    <div class="panel" role="dialog" [attr.aria-label]="t('picker.title')">
      <header>
        <h3>{{ t('picker.title') }}</h3>
        <button class="btn btn-sm" (click)="close.emit()" [attr.aria-label]="t('common.close')">✕</button>
      </header>

      <div class="tree" role="tree">
        @for (node of nodes(); track node.path) {
          <div class="node" role="treeitem" [class.on]="selected() === node.path"
               [class.ghost]="node.ghost" [style.padding-left.px]="8 + node.depth * 16"
               (click)="select(node.path)" (dblclick)="toggle(node)">
            <button class="twist" [class.hidden]="node.leaf"
                    (click)="toggle(node); $event.stopPropagation()"
                    [attr.aria-label]="node.open ? t('picker.collapse') : t('picker.expand')">
              @if (node.loading) { <span class="spin">·</span> } @else { {{ node.open ? '⌄' : '›' }} }
            </button>
            <span class="folder" aria-hidden="true">{{ node.open ? '📂' : '📁' }}</span>
            <span class="name mono">{{ node.name }}</span>
            @if (node.ghost) { <span class="mark">{{ t('picker.willBeCreated') }}</span> }
          </div>
        }
        @if (!nodes().length) {
          <div class="empty">{{ busy() ? t('common.loading') : t('picker.empty') }}</div>
        }
      </div>

      @if (error(); as e) {
        <div class="alert bad">{{ e }}</div>
      }

      @if (naming()) {
        <div class="make">
          <input class="mono" [ngModel]="newName()" (ngModelChange)="newName.set($event)"
                 [placeholder]="t('picker.newFolder')" (keydown.enter)="make()">
          <button class="btn btn-sm btn-primary" (click)="make()" [disabled]="!newName().trim() || busy()">
            {{ t('picker.create') }}
          </button>
        </div>
      }

      <footer>
        <span class="path mono">{{ selected() || t('picker.nothingPicked') }}</span>
        <div class="actions">
          <button class="btn" (click)="startNaming()" [disabled]="!selected() || naming()">
            {{ t('picker.new') }}
          </button>
          <button class="btn" (click)="close.emit()">{{ t('common.cancel') }}</button>
          <button class="btn btn-primary" (click)="pick()" [disabled]="!selected()">
            {{ t('picker.ok') }}
          </button>
        </div>
      </footer>
    </div>
  `,
  styles: `
    .backdrop { position: fixed; inset: 0; background: rgba(4, 5, 8, .72); z-index: 50; }
    .panel {
      position: fixed;
      z-index: 51;
      top: 50%; left: 50%;
      transform: translate(-50%, -50%);
      width: min(520px, calc(100vw - 48px));
      max-height: calc(100vh - 96px);
      display: flex;
      flex-direction: column;
      gap: 12px;
      padding: 16px 18px;
      background: var(--bg-raised);
      border: 1px solid var(--line-strong);
      border-radius: var(--r-lg);
      box-shadow: 0 24px 60px rgba(0, 0, 0, .55);
    }
    header { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
    h3 { margin: 0; font: 600 var(--fs-lg) var(--sans); }

    .tree {
      flex: 1;
      min-height: 240px;
      max-height: 46vh;
      overflow: auto;
      padding: 4px 0;
      background: var(--bg-input);
      border: 1px solid var(--line-strong);
      border-radius: var(--r-sm);
    }
    .node {
      display: flex;
      align-items: center;
      gap: 6px;
      padding-right: 10px;
      height: 24px;
      font-size: var(--fs-sm);
      color: var(--fg-soft);
      white-space: nowrap;
    }
    .node:hover { background: #171d27; }
    .node.on { background: var(--accent-bg); color: var(--accent); }
    .node.ghost .name { font-style: italic; color: var(--fg-dim); }
    .twist {
      width: 14px;
      flex: none;
      color: var(--fg-faint);
      font-size: var(--fs-xs);
      line-height: 1;
    }
    .twist.hidden { visibility: hidden; }
    .spin { color: var(--fg-faint); }
    .folder { flex: none; font-size: 12px; }
    .name { overflow: hidden; text-overflow: ellipsis; }
    .mark { font-size: var(--fs-2xs); color: var(--fg-faint); }
    .empty { padding: 12px 10px; font-size: var(--fs-sm); color: var(--fg-dim); }

    .make { display: flex; gap: 8px; align-items: center; }
    .make input { flex: 1; }
    footer { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
    .path { font-size: var(--fs-2xs); color: var(--fg-faint); overflow: hidden; text-overflow: ellipsis; }
    .actions { display: flex; gap: 8px; flex: none; }
    .alert.bad {
      padding: 8px 10px;
      border-radius: var(--r-sm);
      background: var(--bad-bg);
      border: 1px solid var(--bad-line);
      color: #f3b3b3;
      font-size: var(--fs-xs);
    }
  `,
})
export class DirPicker implements OnInit {
  private readonly api = inject(Api);
  private readonly i18n = inject(I18n);
  readonly t = this.i18n.t;

  readonly start = input('');
  readonly ghosts = input<GhostDir[]>([]);

  readonly picked = output<string>();
  readonly close = output<void>();

  readonly roots = signal<string[]>([]);
  readonly children = signal<Record<string, DirEntry[]>>({});
  readonly open = signal<string[]>([]);
  readonly loading = signal<string[]>([]);
  readonly selected = signal('');
  readonly error = signal('');
  readonly naming = signal(false);
  readonly newName = signal('');

  readonly busy = computed(() => this.loading().length > 0);

  // one flat list, indented: what is on disk plus what this instance is about to be given
  readonly nodes = computed<TreeNode[]>(() => {
    const out: TreeNode[] = [];
    const walk = (path: string, name: string, depth: number, ghost: boolean) => {
      const open = this.open().includes(path);
      const kids = this.childrenOf(path);
      out.push({
        name,
        path,
        depth,
        ghost,
        open,
        loading: this.loading().includes(path),
        leaf: ghost && !kids.length,
      });
      if (!open) return;
      for (const kid of kids) walk(kid.path, kid.name, depth + 1, this.isGhost(kid.path));
    };
    for (const root of this.roots()) walk(root, root, 0, false);
    return out;
  });

  ngOnInit(): void {
    this.load(this.start(), true);
  }

  toggle(node: TreeNode): void {
    if (this.open().includes(node.path)) {
      this.open.update((list) => list.filter((p) => p !== node.path));
      return;
    }
    this.open.update((list) => [...list, node.path]);
    if (!this.children()[node.path] && !this.isGhost(node.path)) this.load(node.path, false);
  }

  select(path: string): void {
    this.selected.set(path);
    this.naming.set(false);
  }

  startNaming(): void {
    this.newName.set('');
    this.naming.set(true);
  }

  make(): void {
    const name = this.newName().trim();
    const parent = this.selected();
    if (!name || !parent) return;
    this.mark(parent, true);
    this.error.set('');
    this.api.makeDir(`${parent}/${name}`).subscribe({
      next: ({ path }) => {
        this.mark(parent, false);
        this.naming.set(false);
        this.newName.set('');
        this.selected.set(path);
        this.load(parent, false);
      },
      error: (err: OkDockError) => {
        this.mark(parent, false);
        this.error.set(err.message);
      },
    });
  }

  pick(): void {
    const path = this.selected();
    if (path) this.picked.emit(path);
  }

  // a folder that is not there yet has nothing to list, its children are ghosts alone
  private load(path: string, first: boolean): void {
    this.mark(path, true);
    this.error.set('');
    this.api.browseDirs(path).subscribe({
      next: (listing) => {
        this.mark(path, false);
        this.mark(listing.path, false);
        if (first) {
          this.roots.set(listing.roots.length ? listing.roots : [listing.path]);
          this.selected.set(listing.path);
          // a folder that is about to be created is what the picker is there for, so it opens shown
          this.open.update((list) => [...list, ...this.ghosts().map((ghost) => ghost.path)]);
        }
        this.children.update((cur) => ({ ...cur, [listing.path]: listing.entries }));
        this.open.update((list) => (list.includes(listing.path) ? list : [...list, listing.path]));
      },
      error: (err: OkDockError) => {
        this.mark(path, false);
        this.error.set(err.message);
      },
    });
  }

  private childrenOf(path: string): DirEntry[] {
    const real = this.children()[path] ?? [];
    const here = new Set(real.map((entry) => entry.path));
    const ghosts = this.ghosts()
      .filter((ghost) => !here.has(ghost.path) && parentOf(ghost.path) === path)
      .map((ghost) => ({ name: ghost.label ?? nameOf(ghost.path), path: ghost.path }));
    return [...real, ...ghosts].sort((a, b) => a.name.localeCompare(b.name));
  }

  private isGhost(path: string): boolean {
    return this.ghosts().some((ghost) => ghost.path === path);
  }

  private mark(path: string, on: boolean): void {
    this.loading.update((list) =>
      on ? (list.includes(path) ? list : [...list, path]) : list.filter((p) => p !== path),
    );
  }
}

function parentOf(path: string): string {
  const cut = path.replace(/\/+$/, '').lastIndexOf('/');
  if (cut <= 0) return '/';
  return path.slice(0, cut);
}

function nameOf(path: string): string {
  return path.replace(/\/+$/, '').split('/').pop() ?? path;
}
