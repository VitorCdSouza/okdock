import { ChangeDetectionStrategy, Component, inject, input, output, signal } from '@angular/core';

import { I18n } from '../core/i18n/i18n';
import { DirPicker, GhostDir, OPEN_FOLDER } from './dir-picker';

@Component({
  selector: 'ok-pick-dir',
  imports: [DirPicker],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <button class="btn" (click)="open.set(true)"
            [title]="t('common.pick')" [attr.aria-label]="t('common.pick')">
      <svg class="folder" viewBox="0 0 16 16" aria-hidden="true">
        <path [attr.d]="openFolder" />
      </svg>
    </button>

    @if (open()) {
      <ok-dir-picker [start]="start()" [ghosts]="ghosts()"
                     (picked)="take($event)" (close)="open.set(false)" />
    }
  `,
  styles: `
    :host { display: inline-flex; }
    .btn { flex: 1; justify-content: center; padding: 0 10px; }
    .folder {
      width: 15px;
      height: 15px;
      fill: none;
      stroke: currentColor;
      stroke-width: 1.2;
      stroke-linecap: round;
      stroke-linejoin: round;
    }
  `,
})
export class PickDir {
  readonly openFolder = OPEN_FOLDER;

  private readonly i18n = inject(I18n);
  readonly t = this.i18n.t;

  readonly start = input('');
  readonly ghosts = input<GhostDir[]>([]);

  readonly picked = output<string>();

  readonly open = signal(false);

  take(path: string): void {
    this.open.set(false);
    this.picked.emit(path);
  }
}
