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
import { Subject, catchError, debounceTime, distinctUntilChanged, map, of, switchMap } from 'rxjs';

import { Api } from '../core/api';
import { ImageHit } from '../core/models';
import { I18n } from '../core/i18n/i18n';
import { splitImage } from './image-ref';

// docker search knows only the Hub and answers repositories, so the tag stays typed
@Component({
  selector: 'ok-image-search',
  imports: [FormsModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  host: {
    '(document:click)': 'hits.set([])',
    '(document:keydown.escape)': 'hits.set([])',
  },
  template: `
    <div class="wrap" (click)="$event.stopPropagation()">
      <input class="mono" spellcheck="false" [attr.id]="fieldId() || null"
             [placeholder]="placeholder()" [ngModel]="image()"
             (ngModelChange)="type($event)" (focus)="type(image())">
      @if (busy()) { <span class="state mono">{{ t('images.searching') }}</span> }
      @else if (failed()) { <span class="state mono bad">{{ t('images.failed') }}</span> }
      @else if (empty()) { <span class="state mono">{{ t('images.none') }}</span> }

      @if (hits().length) {
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
    input { width: 100%; }
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

  readonly hits = signal<ImageHit[]>([]);
  readonly busy = signal(false);
  readonly failed = signal(false);
  private readonly searched = signal('');

  readonly empty = computed(
    () => this.searched() !== '' && !this.failed() && this.hits().length === 0,
  );

  private readonly typed = new Subject<string>();

  constructor() {
    this.typed
      .pipe(
        debounceTime(300),
        distinctUntilChanged(),
        switchMap((term) => {
          if (term.length < 2) {
            this.reset();
            return of({ term: '', hits: [] as ImageHit[], failed: false });
          }
          this.busy.set(true);
          // the catch belongs inside, an error escaping here would kill the stream for good
          return this.api.searchImages(term).pipe(
            map((hits) => ({ term, hits, failed: false })),
            catchError(() => of({ term, hits: [] as ImageHit[], failed: true })),
          );
        }),
        takeUntilDestroyed(inject(DestroyRef)),
      )
      .subscribe({
        next: ({ term, hits, failed }) => {
          this.busy.set(false);
          this.searched.set(term);
          this.failed.set(failed);
          this.hits.set(hits);
        },
      });
  }

  type(raw: string): void {
    this.image.set(raw);
    // the registry searches a repository, so a tag already typed is not part of the term
    this.typed.next(splitImage(raw.trim()).repo);
  }

  pick(hit: ImageHit): void {
    const tag = splitImage(this.image().trim()).tag;
    this.image.set(tag ? `${hit.name}:${tag}` : hit.name);
    this.reset();
  }

  private reset(): void {
    this.busy.set(false);
    this.failed.set(false);
    this.hits.set([]);
    this.searched.set('');
  }
}
