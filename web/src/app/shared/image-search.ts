import {
  ChangeDetectionStrategy,
  Component,
  DestroyRef,
  computed,
  inject,
  input,
  model,
  signal,
} from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { FormsModule } from '@angular/forms';
import {
  Observable,
  Subject,
  catchError,
  debounceTime,
  distinctUntilChanged,
  map,
  of,
  switchMap,
} from 'rxjs';

import { Api } from '../core/api';
import { ImageHit } from '../core/models';
import { I18n } from '../core/i18n/i18n';
import { isHubRepo, splitImage } from './image-ref';

type Result =
  | { kind: 'repo'; term: string; hits: ImageHit[]; failed: boolean }
  | { kind: 'tag'; tags: string[]; failed: boolean }
  | { kind: 'idle'; failed: boolean };

const IDLE: Result = { kind: 'idle', failed: false };

type Open = 'repo' | 'tag' | null;

// docker search answers repositories and the Hub answers tags, so the image comes first
@Component({
  selector: 'ok-image-search',
  imports: [FormsModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  host: {
    '(document:click)': 'close()',
    '(document:keydown.escape)': 'close()',
  },
  template: `
    <div class="wrap" (click)="$event.stopPropagation()">
      <div class="fields">
        <span class="box grow">
          <input class="mono" spellcheck="false" [attr.id]="fieldId() || null"
                 [placeholder]="placeholder()" [ngModel]="repo()"
                 (ngModelChange)="typeRepo($event)">
          @if (open() === 'repo') {
            @if (busy()) { <span class="state mono">{{ t('images.searching') }}</span> }
            @else if (failed()) { <span class="state mono bad">{{ t('images.failed') }}</span> }
            @else if (empty()) { <span class="state mono">{{ t('images.none') }}</span> }
          }
        </span>

        <span class="box version">
          <label class="cap mono" [attr.for]="fieldId() ? fieldId() + '-version' : null">
            {{ t('images.version') }}
          </label>
          <input class="mono" spellcheck="false" placeholder="latest"
                 [attr.id]="fieldId() ? fieldId() + '-version' : null"
                 [disabled]="!repo()" [ngModel]="tag()"
                 (ngModelChange)="typeTag($event)" (focus)="openTags()">
          @if (open() === 'tag') {
            @if (busy()) { <span class="state mono">{{ t('images.searching') }}</span> }
            @else if (failed()) { <span class="state mono bad">{{ t('images.failed') }}</span> }
            @else if (notHub()) { <span class="state mono">{{ t('images.tagsOnlyHub') }}</span> }
          }
        </span>
      </div>

      @if (open() === 'tag' && tags().length) {
        <ul class="hits tags" role="listbox">
          @for (tag of tags(); track tag) {
            <li>
              <button type="button" role="option" (click)="pickTag(tag)">
                <span class="name mono">{{ tag }}</span>
              </button>
            </li>
          }
        </ul>
      } @else if (open() === 'repo' && hits().length) {
        <ul class="hits" role="listbox">
          @for (hit of hits(); track hit.name) {
            <li>
              <button type="button" role="option" (click)="pick(hit)">
                <span class="name mono">{{ hit.name }}</span>
                @if (hit.official) { <span class="tag">{{ t('images.official') }}</span> }
                <span class="stars mono">★ {{ hit.stars }}</span>
                @if (hit.description) { <span class="desc">{{ hit.description }}</span> }
              </button>
            </li>
          }
        </ul>
      }
    </div>
  `,
  styles: `
    .wrap { position: relative; }
    .fields { display: flex; gap: 6px; align-items: stretch; }
    .box { position: relative; display: block; }
    .grow { flex: 1; min-width: 0; }
    .version { flex: none; width: 168px; }
    .box input { width: 100%; }
    .box input:disabled { opacity: .5; cursor: not-allowed; }
    .cap {
      position: absolute;
      top: 50%;
      left: 9px;
      transform: translateY(-50%);
      font-size: var(--fs-2xs);
      color: var(--fg-faint);
      pointer-events: none;
    }
    .version input { padding-left: 58px; }
    .state {
      position: absolute;
      right: 8px;
      top: 50%;
      transform: translateY(-50%);
      font-size: var(--fs-2xs);
      color: var(--fg-faint);
      pointer-events: none;
    }
    .state.bad { color: var(--bad); }
    .hits {
      position: absolute;
      z-index: 20;
      top: calc(100% + 4px);
      left: 0;
      right: 0;
      max-height: 260px;
      overflow-y: auto;
      margin: 0;
      padding: 4px;
      list-style: none;
      background: var(--bg-header);
      border: 1px solid var(--line-strong);
      border-radius: var(--r-sm);
      box-shadow: 0 8px 24px rgba(0, 0, 0, .5);
    }
    .hits.tags { left: auto; width: 168px; }
    .hits button {
      display: grid;
      grid-template-columns: 1fr auto auto;
      gap: 2px 8px;
      width: 100%;
      padding: 6px 8px;
      text-align: left;
      border-radius: var(--r-xs);
    }
    .hits button:hover { background: var(--bg-chip); }
    .name { font-size: var(--fs-sm); color: var(--fg); overflow: hidden; text-overflow: ellipsis; }
    .stars { font-size: var(--fs-2xs); color: var(--fg-faint); }
    .tag {
      font-size: var(--fs-2xs);
      color: var(--ok);
      border: 1px solid var(--ok-line);
      border-radius: var(--r-xs);
      padding: 0 4px;
    }
    .desc {
      grid-column: 1 / -1;
      font-size: var(--fs-2xs);
      color: var(--fg-dim);
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }
  `,
})
export class ImageSearch {
  private readonly api = inject(Api);
  private readonly i18n = inject(I18n);
  readonly t = this.i18n.t;

  readonly image = model.required<string>();
  readonly placeholder = input('');
  readonly fieldId = input('');

  // an explicit open, or a reply that lands late reopens what the user just closed
  readonly open = signal<Open>(null);
  readonly hits = signal<ImageHit[]>([]);
  readonly busy = signal(false);
  readonly failed = signal(false);
  readonly notHub = signal(false);

  readonly repo = computed(() => splitImage(this.image()).repo);
  readonly tag = computed(() => splitImage(this.image()).tag);

  private readonly searched = signal('');
  private readonly allTags = signal<string[]>([]);

  readonly empty = computed(
    () => this.searched() !== '' && !this.failed() && this.hits().length === 0,
  );

  // the tag list is fetched once per repository and filtered here, so the version box is free
  readonly tags = computed(() => {
    const prefix = this.tag().toLowerCase();
    return this.allTags()
      .filter((tag) => tag.toLowerCase().includes(prefix))
      .slice(0, 40);
  });

  private readonly typed = new Subject<string>();

  constructor() {
    this.typed
      .pipe(
        debounceTime(300),
        distinctUntilChanged(),
        switchMap((key) => this.load(key)),
        takeUntilDestroyed(inject(DestroyRef)),
      )
      .subscribe((res) => {
        this.busy.set(false);
        this.failed.set(res.failed);
        if (res.kind === 'repo') {
          this.searched.set(res.term);
          this.hits.set(res.hits);
        } else if (res.kind === 'tag') {
          this.allTags.set(res.tags);
        }
      });
  }

  // the catch belongs inside the switchMap, an error escaping it kills the stream for good
  private load(key: string): Observable<Result> {
    const cut = key.indexOf(':');
    const [kind, term] = [key.slice(0, cut), key.slice(cut + 1)];
    if (kind === 'tag') {
      if (!isHubRepo(term)) {
        this.notHub.set(true);
        this.allTags.set([]);
        return of(IDLE);
      }
      this.notHub.set(false);
      this.busy.set(true);
      return this.api.imageTags(term).pipe(
        map((tags) => ({ kind: 'tag', tags, failed: false }) as Result),
        catchError(() => of({ kind: 'tag', tags: [], failed: true } as Result)),
      );
    }
    if (term.length < 2) {
      this.reset();
      return of(IDLE);
    }
    this.busy.set(true);
    return this.api.searchImages(term).pipe(
      map((hits) => ({ kind: 'repo', term, hits, failed: false }) as Result),
      catchError(() => of({ kind: 'repo', term, hits: [], failed: true } as Result)),
    );
  }

  // a whole reference pasted into the image box splits itself
  typeRepo(raw: string): void {
    const { repo, tag } = splitImage(raw.trim());
    this.setRef(repo, tag || this.tag());
    this.open.set('repo');
    this.allTags.set([]);
    this.notHub.set(false);
    this.typed.next(`repo:${repo}`);
  }

  typeTag(raw: string): void {
    this.setRef(this.repo(), raw.trim());
    this.openTags();
  }

  openTags(): void {
    const repo = this.repo();
    if (!repo) return;
    this.open.set('tag');
    this.hits.set([]);
    this.typed.next(`tag:${repo}`);
  }

  // picking the repository is half of the reference, so the version box is offered next
  pick(hit: ImageHit): void {
    this.setRef(hit.name, this.tag());
    this.hits.set([]);
    this.openTags();
  }

  pickTag(tag: string): void {
    this.setRef(this.repo(), tag);
    this.open.set(null);
  }

  close(): void {
    this.open.set(null);
  }

  private setRef(repo: string, tag: string): void {
    this.image.set(tag && repo ? `${repo}:${tag}` : repo);
  }

  private reset(): void {
    this.busy.set(false);
    this.searched.set('');
    this.failed.set(false);
    this.notHub.set(false);
    this.hits.set([]);
    this.allTags.set([]);
  }
}
