import { ChangeDetectionStrategy, Component, computed, input, model } from '@angular/core';
import { FormsModule } from '@angular/forms';

export function splitImage(ref: string): { repo: string; tag: string } {
  const slash = ref.lastIndexOf('/');
  const colon = ref.lastIndexOf(':');
  if (colon > slash) return { repo: ref.slice(0, colon), tag: ref.slice(colon + 1) };
  return { repo: ref, tag: '' };
}

@Component({
  selector: 'gd-image-ref',
  imports: [FormsModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    @if (free()) {
      <input class="mono whole" [ngModel]="image()" (ngModelChange)="image.set($event)"
             placeholder="repositorio/imagem:tag" spellcheck="false">
    } @else {
      <div class="ref">
        <span class="repo mono" [title]="repo()">{{ repo() }}:</span>
        <input class="mono tag" [attr.list]="listId()" [ngModel]="tag()"
               (ngModelChange)="setTag($event)" placeholder="latest" spellcheck="false">
        @if (tags().length) {
          <datalist [id]="listId()">
            @for (t of tags(); track t) { <option [value]="t"></option> }
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
  readonly image = model.required<string>();
  readonly tags = input<string[]>([]);
  readonly free = input(false);
  readonly listId = input('image-tags');

  readonly repo = computed(() => splitImage(this.image()).repo);
  readonly tag = computed(() => splitImage(this.image()).tag);

  setTag(raw: string): void {
    const tag = raw.trim();
    this.image.set(tag ? `${this.repo()}:${tag}` : this.repo());
  }
}
