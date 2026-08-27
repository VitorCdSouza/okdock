import { ChangeDetectionStrategy, Component, inject, input, output, signal } from '@angular/core';

import { I18n } from '../core/i18n/i18n';
import { DirPicker, GhostDir } from './dir-picker';

@Component({
  selector: 'ok-pick-dir',
  imports: [DirPicker],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <button class="btn btn-sm" (click)="open.set(true)">{{ t('common.pick') }}</button>

    @if (open()) {
      <ok-dir-picker [start]="start()" [ghosts]="ghosts()"
                     (picked)="take($event)" (close)="open.set(false)" />
    }
  `,
  styles: `
    :host { display: inline-flex; }
    .btn { flex: 1; justify-content: center; }
  `,
})
export class PickDir {
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
