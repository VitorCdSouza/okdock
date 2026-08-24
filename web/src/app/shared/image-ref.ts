import {
  ChangeDetectionStrategy,
  Component,
  DestroyRef,
  computed,
  effect,
  inject,
  input,
  model,
  signal,
} from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { FormsModule } from '@angular/forms';

import { Api } from '../core/api';
import { ImageSearch } from './image-search';

// the tag list comes from the Hub API, and a first element that looks like a host is not it
export function isHubRepo(repo: string): boolean {
  const parts = repo.trim().split('/');
  if (parts.length === 1) return parts[0] !== '';
  if (parts.length > 2) return false;
  return parts[0] !== 'localhost' && !/[.:]/.test(parts[0]);
}

export function splitImage(ref: string): { repo: string; tag: string } {
  const slash = ref.lastIndexOf('/');
  const colon = ref.lastIndexOf(':');
  if (colon > slash) return { repo: ref.slice(0, colon), tag: ref.slice(colon + 1) };
  return { repo: ref, tag: '' };
}

@Component({
  selector: 'ok-image-ref',
  imports: [FormsModule, ImageSearch],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    @if (free()) {
      <ok-image-search [(image)]="image" [label]="label()" [tip]="tip()"
                       placeholder="repositorio/imagem" />
    } @else {
      <div class="ref">
        <span class="repo mono" [title]="repo()">{{ repo() }}:</span>
        <input class="mono tag" [attr.list]="listId()" [ngModel]="tag()"
               (ngModelChange)="setTag($event)" placeholder="latest" spellcheck="false">
        @if (options().length) {
          <datalist [id]="listId()">
            @for (t of options(); track t) { <option [value]="t"></option> }
          </datalist>
        }
      </div>
    }
  `,
  styles: `
    .ref {
      display: flex;
      align-items: stretch;
      background: var(--bg-input);
      border: 1px solid var(--line-strong);
      border-radius: var(--r-sm);
      overflow: hidden;
    }
    .repo {
      display: flex;
      align-items: center;
      padding: 0 0 0 9px;
      font-size: var(--fs-sm);
      color: var(--fg-dim);
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
      max-width: 60%;
    }
    .tag {
      flex: 1;
      min-width: 60px;
      border: none;
      background: none;
      color: var(--fg);
      font-size: var(--fs-sm);
      padding: 7px 9px 7px 0;
    }
    .tag:focus { outline: none; }
    .ref:focus-within { border-color: var(--accent-line); }
    .whole { width: 100%; }
  `,
})
export class ImageRef {
  private readonly api = inject(Api);
  private readonly destroyRef = inject(DestroyRef);

  readonly image = model.required<string>();
  readonly tags = input<string[]>([]);
  readonly free = input(false);
  readonly label = input('');
  readonly tip = input('');
  readonly listId = input('image-tags');

  readonly repo = computed(() => splitImage(this.image()).repo);
  readonly tag = computed(() => splitImage(this.image()).tag);

  private readonly fetched = signal<string[]>([]);
  private asked = '';

  // the template says which tags it was written for, and the Hub answers when it says nothing
  readonly options = computed(() => (this.tags().length ? this.tags() : this.fetched()));

  constructor() {
    effect(() => {
      const repo = this.repo();
      if (this.free() || this.tags().length || repo === this.asked) return;
      this.asked = repo;
      this.fetched.set([]);
      if (!isHubRepo(repo)) return;
      this.api
        .imageTags(repo)
        .pipe(takeUntilDestroyed(this.destroyRef))
        .subscribe({ next: (tags) => this.fetched.set(tags), error: () => {} });
    });
  }

  setTag(raw: string): void {
    const tag = raw.trim();
    this.image.set(tag ? `${this.repo()}:${tag}` : this.repo());
  }
}
