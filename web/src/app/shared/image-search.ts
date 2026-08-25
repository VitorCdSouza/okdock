import {
  ChangeDetectionStrategy,
  Component,
  DestroyRef,
  ElementRef,
  computed,
  effect,
  inject,
  input,
  model,
  output,
  signal,
  viewChild,
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
import { InfoDot } from './info-dot';

type Result =
  | { kind: 'repo'; term: string; hits: ImageHit[]; failed: boolean }
  | { kind: 'tag'; tags: string[]; failed: boolean }
  | { kind: 'idle'; failed: boolean };

const IDLE: Result = { kind: 'idle', failed: false };

type Open = 'repo' | 'tag' | null;

// where the list is drawn, in viewport coordinates
interface Box {
  left: number;
  width: number;
  maxHeight: number;
  top?: number;
  bottom?: number;
}

// a tag is never wrapped over two lines, so the list is free to be wider than the box
const TAG_LIST_MIN = 260;

// docker search answers repositories and the Hub answers tags, so the image comes first
@Component({
  selector: 'ok-image-search',
  imports: [FormsModule, InfoDot],
  changeDetection: ChangeDetectionStrategy.OnPush,
  host: {
    '(document:click)': 'close()',
    '(document:keydown.escape)': 'close()',
  },
  template: `
    <div class="wrap" (click)="$event.stopPropagation()">
      <div class="fields">
        <span class="box grow">
          <label [attr.for]="fieldId() || null">
            {{ label() || t('images.image') }}
            @if (required()) { <span class="req">*</span> }
            @if (tip()) { <ok-info [text]="tip()" /> }
          </label>
          <span class="entry" #repoEntry>
            <input class="mono" spellcheck="false" [attr.id]="fieldId() || null"
                   [class.locked]="lockRepo()" [readonly]="lockRepo()"
                   [placeholder]="placeholder()" [ngModel]="repo()"
                   (ngModelChange)="typeRepo($event)">
            @if (open() === 'repo' && busy()) {
              <span class="state mono">{{ t('images.searching') }}</span>
            } @else if (open() === 'repo' && failed()) {
              <span class="state mono bad">{{ t('images.failed') }}</span>
            } @else if (open() === 'repo' && empty()) {
              <span class="state mono">{{ t('images.none') }}</span>
            }
            @if (!lockRepo()) {
              <button type="button" class="caret" tabindex="-1"
                      [attr.aria-label]="t('images.openList')" (click)="openRepos()">
                <svg viewBox="0 0 10 6" aria-hidden="true">
                  <path d="M1 1l4 4 4-4" />
                </svg>
              </button>
            }
          </span>

          @if (open() === 'repo' && hits().length) {
            <ul #list class="hits" role="listbox" popover="manual"
                [style.left.px]="box().left" [style.width.px]="box().width"
                [style.top.px]="box().top" [style.bottom.px]="box().bottom"
                [style.max-height.px]="box().maxHeight">
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
        </span>

        <span class="box version">
          <label [attr.for]="versionId()">{{ t('images.version') }}</label>
          <span class="entry" #tagEntry>
            <input class="mono" spellcheck="false" placeholder="latest"
                   [attr.id]="versionId()" [disabled]="!validImage()" [ngModel]="tag()"
                   (ngModelChange)="typeTag($event)" (focus)="openTags()">
            @if (open() === 'tag' && busy()) {
              <span class="state mono">{{ t('images.searching') }}</span>
            } @else if (open() === 'tag' && failed()) {
              <span class="state mono bad">{{ t('images.failed') }}</span>
            } @else if (open() === 'tag' && notHub()) {
              <span class="state mono">{{ t('images.tagsOnlyHub') }}</span>
            }
            <button type="button" class="caret" tabindex="-1" [disabled]="!validImage()"
                    [attr.aria-label]="t('images.openList')" (click)="openTags()">
              <svg viewBox="0 0 10 6" aria-hidden="true">
                <path d="M1 1l4 4 4-4" />
              </svg>
            </button>
          </span>

          @if (open() === 'tag' && tags().length) {
            <ul #list class="hits" role="listbox" popover="manual"
                [style.left.px]="box().left" [style.width.px]="box().width"
                [style.top.px]="box().top" [style.bottom.px]="box().bottom"
                [style.max-height.px]="box().maxHeight">
              @for (tag of tags(); track tag) {
                <li>
                  <button type="button" role="option" (click)="pickTag(tag)">
                    <span class="name mono">{{ tag }}</span>
                  </button>
                </li>
              }
            </ul>
          }
        </span>
      </div>
    </div>
  `,
  styles: `
    .wrap { position: relative; }
    .fields { display: flex; gap: 8px; align-items: flex-end; }
    .box { position: relative; display: flex; flex-direction: column; gap: 4px; min-width: 0; }
    .grow { flex: 1; }
    .version { flex: 0 1 320px; min-width: 140px; }
    label {
      display: flex;
      align-items: center;
      gap: 5px;
      font-size: var(--fs-xs);
      color: var(--fg-muted);
    }
    .req { color: var(--bad); }

    .entry { position: relative; display: block; }
    .entry input { width: 100%; padding-right: 30px; }
    .entry input:disabled { opacity: .5; cursor: not-allowed; }
    .entry input.locked { color: var(--fg-dim); padding-right: 9px; }

    .caret {
      position: absolute;
      right: 2px;
      top: 50%;
      transform: translateY(-50%);
      width: 24px;
      height: 24px;
      display: grid;
      place-items: center;
      border-radius: var(--r-xs);
      font-size: var(--fs-xs);
      color: var(--fg-faint);
    }
    .caret svg {
      width: 10px;
      height: 6px;
      fill: none;
      stroke: currentColor;
      stroke-width: 1.6;
      stroke-linecap: round;
      stroke-linejoin: round;
    }
    .caret:hover:not(:disabled) { color: var(--fg); background: var(--bg-chip); }
    .caret:disabled { opacity: .4; cursor: not-allowed; }

    .state {
      position: absolute;
      right: 28px;
      top: 50%;
      transform: translateY(-50%);
      font-size: var(--fs-2xs);
      color: var(--fg-faint);
      pointer-events: none;
    }
    .state.bad { color: var(--bad); }

    /* the top layer is what keeps the list out of the overflow of a dialog */
    .hits {
      position: fixed;
      inset: auto;
      z-index: 60;
      display: block;
      overflow-y: auto;
      overscroll-behavior: contain;
      margin: 0;
      padding: 4px;
      list-style: none;
      background: var(--bg-header);
      border: 1px solid var(--line-strong);
      border-radius: var(--r-sm);
      box-shadow: 0 8px 24px rgba(0, 0, 0, .5);
      --scroll-bg: var(--bg-header);
      scrollbar-color: var(--line-strong) var(--bg-header);
    }
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
    .name {
      font-size: var(--fs-sm);
      color: var(--fg);
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
    }
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
  readonly label = input('');
  readonly tip = input('');
  readonly required = input(false);
  // a template already says the image, so only the version stays open
  readonly lockRepo = input(false);

  // a reference that exists, which is the moment a form can be filled from it
  readonly picked = output<string>();

  readonly versionId = computed(() => (this.fieldId() ? `${this.fieldId()}-version` : null));

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

  // the version means something over an image that exists, picked from the list or found by search
  private readonly typedByHand = signal(false);
  private readonly confirmed = signal('');

  readonly validImage = computed(() => {
    const repo = this.repo();
    if (!repo) return false;
    if (this.lockRepo()) return true;
    if (!this.typedByHand() || !isHubRepo(repo)) return true;
    return this.confirmed() === repo;
  });

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

  readonly box = signal<Box>({ left: 0, width: 0, maxHeight: 260 });

  private readonly repoEntry = viewChild<ElementRef<HTMLElement>>('repoEntry');
  private readonly tagEntry = viewChild<ElementRef<HTMLElement>>('tagEntry');
  private readonly list = viewChild<ElementRef<HTMLElement>>('list');

  constructor() {
    effect((onCleanup) => {
      const list = this.list()?.nativeElement;
      if (!list) return;
      this.place();
      if (!list.matches(':popover-open')) list.showPopover?.();
      // out of the flow, the list only follows the field if something moves it
      const follow = () => this.place();
      window.addEventListener('scroll', follow, true);
      window.addEventListener('resize', follow);
      onCleanup(() => {
        window.removeEventListener('scroll', follow, true);
        window.removeEventListener('resize', follow);
      });
    });

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
          if (res.hits.some((hit) => hit.name === res.term)) {
            this.confirmed.set(res.term);
            this.picked.emit(this.image());
          }
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

  // a whole reference pasted into the image box splits itself, tag of the old repository included
  typeRepo(raw: string): void {
    if (this.lockRepo()) return;
    const { repo, tag } = splitImage(raw.trim());
    this.setRef(repo, tag);
    this.typedByHand.set(true);
    this.open.set('repo');
    this.allTags.set([]);
    this.notHub.set(false);
    this.typed.next(`repo:${repo}`);
  }

  typeTag(raw: string): void {
    this.setRef(this.repo(), raw.trim());
    this.openTags();
  }

  // the caret is the affordance for a list that is not a datalist, and opens what typing opens
  openRepos(): void {
    this.open.set('repo');
    this.allTags.set([]);
    this.typed.next(`repo:${this.repo()}`);
  }

  openTags(): void {
    const repo = this.repo();
    if (!this.validImage()) return;
    this.open.set('tag');
    this.hits.set([]);
    this.typed.next(`tag:${repo}`);
  }

  // picking the repository is half of the reference, so the version box is offered next
  pick(hit: ImageHit): void {
    this.setRef(hit.name, this.tag());
    this.confirmed.set(hit.name);
    this.typedByHand.set(false);
    this.hits.set([]);
    this.picked.emit(this.image());
    this.openTags();
  }

  pickTag(tag: string): void {
    this.setRef(this.repo(), tag);
    this.open.set(null);
    this.picked.emit(this.image());
  }

  close(): void {
    this.open.set(null);
  }

  // opens downwards, unless what is left under the field is the smaller half
  private place(): void {
    const kind = this.open();
    const anchor = (kind === 'tag' ? this.tagEntry() : this.repoEntry())?.nativeElement;
    if (!anchor) return;
    const rect = anchor.getBoundingClientRect();
    const gap = 4;
    const edge = 12;
    const below = window.innerHeight - rect.bottom - gap - edge;
    const above = rect.top - gap - edge;
    const up = below < Math.min(200, above);
    const room = Math.max(120, up ? above : below);
    const wanted = kind === 'tag' ? Math.max(rect.width, TAG_LIST_MIN) : rect.width;
    const width = Math.min(wanted, window.innerWidth - 2 * edge);
    const left = Math.max(edge, Math.min(rect.left, window.innerWidth - width - edge));
    this.box.set({
      left,
      width,
      maxHeight: Math.min(260, room),
      top: up ? undefined : rect.bottom + gap,
      bottom: up ? window.innerHeight - rect.top + gap : undefined,
    });
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
