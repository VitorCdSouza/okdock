import {
  ChangeDetectionStrategy,
  Component,
  DestroyRef,
  inject,
  input,
  signal,
} from '@angular/core';
import { copyText } from '../core/clipboard';
import { I18n } from '../core/i18n/i18n';

// sits in the top right corner of the block around it, which carries the copyable class
@Component({
  selector: 'ok-copy',
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <button type="button" (click)="copy()"
            [attr.aria-label]="done() ? t('common.copied') : t('common.copy')"
            [title]="done() ? t('common.copied') : t('common.copy')">
      @if (done()) {
        <svg viewBox="0 0 16 16" aria-hidden="true"><path d="M3.5 8.5l3 3 6-7" /></svg>
      } @else {
        <svg viewBox="0 0 16 16" aria-hidden="true">
          <rect x="5.5" y="5.5" width="8" height="8" rx="1.5" />
          <path d="M10.5 3.5v-.5a1 1 0 0 0-1-1h-6a1 1 0 0 0-1 1v6a1 1 0 0 0 1 1h.5" />
        </svg>
      }
    </button>
  `,
  styles: `
    :host { position: absolute; top: 6px; right: 6px; }
    button {
      display: grid;
      place-items: center;
      width: 22px; height: 22px;
      padding: 0;
      border: 1px solid transparent;
      border-radius: var(--r-sm);
      background: transparent;
      color: inherit;
      opacity: .7;
      cursor: pointer;
    }
    button:hover, button:focus-visible { opacity: 1; border-color: currentColor; outline: none; }
    svg { width: 14px; height: 14px; fill: none; stroke: currentColor; stroke-width: 1.4; stroke-linecap: round; stroke-linejoin: round; }
  `,
})
export class CopyButton {
  readonly t = inject(I18n).t;
  readonly text = input.required<string>();
  readonly done = signal(false);

  private timer?: ReturnType<typeof setTimeout>;

  constructor() {
    inject(DestroyRef).onDestroy(() => clearTimeout(this.timer));
  }

  copy(): void {
    copyText(this.text());
    this.done.set(true);
    clearTimeout(this.timer);
    this.timer = setTimeout(() => this.done.set(false), 1500);
  }
}
