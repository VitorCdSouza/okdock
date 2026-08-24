import {
  ChangeDetectionStrategy,
  Component,
  computed,
  input,
  model,
  signal,
} from '@angular/core';

export interface SelectOption {
  value: string;
  label: string;
}

// the native option list cannot be styled, and a select would look like another kind of field
@Component({
  selector: 'ok-select',
  changeDetection: ChangeDetectionStrategy.OnPush,
  host: {
    '(document:click)': 'open.set(false)',
    '(document:keydown.escape)': 'open.set(false)',
  },
  template: `
    <div class="wrap" (click)="$event.stopPropagation()">
      <button type="button" class="trigger mono" role="combobox"
              [attr.id]="fieldId() || null" [attr.aria-expanded]="open()"
              [attr.aria-label]="ariaLabel() || null" [disabled]="disabled()"
              [class.invalid]="invalid()" (click)="toggle()" (keydown)="onKey($event)">
        <span class="label" [class.none]="!current()">{{ current()?.label ?? placeholder() }}</span>
        <svg class="caret" viewBox="0 0 10 6" aria-hidden="true">
          <path d="M1 1l4 4 4-4" />
        </svg>
      </button>

      @if (open()) {
        <ul class="list" role="listbox">
          @for (option of options(); track option.value; let i = $index) {
            <li>
              <button type="button" role="option" class="mono"
                      [class.on]="option.value === value()"
                      [class.active]="i === active()"
                      [attr.aria-selected]="option.value === value()"
                      (click)="pick(option)" (mouseenter)="active.set(i)">
                {{ option.label }}
              </button>
            </li>
          }
        </ul>
      }
    </div>
  `,
  styles: `
    :host { display: block; min-width: 0; }
    .wrap { position: relative; }

    .trigger {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 8px;
      width: 100%;
      padding: 7px 9px;
      font-size: var(--fs-md);
      color: var(--fg);
      background: var(--bg-input);
      border: 1px solid var(--line-strong);
      border-radius: var(--r-sm);
      cursor: pointer;
      text-align: left;
    }
    .trigger:focus-visible, .trigger[aria-expanded='true'] {
      outline: none;
      border-color: var(--accent-line);
    }
    .trigger:disabled { opacity: .5; cursor: not-allowed; }
    /* the parent cannot reach in here, so the error state comes as an input */
    .trigger.invalid { border-color: var(--bad-line); }
    .label { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
    .label.none { color: var(--fg-faint); }

    .caret {
      flex: none;
      width: 10px;
      height: 6px;
      fill: none;
      stroke: var(--fg-faint);
      stroke-width: 1.6;
      stroke-linecap: round;
      stroke-linejoin: round;
    }

    .list {
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
    .list button {
      width: 100%;
      padding: 6px 8px;
      text-align: left;
      border-radius: var(--r-xs);
      font-size: var(--fs-sm);
      color: var(--fg-soft);
    }
    .list button.active { background: var(--bg-chip); color: var(--fg); }
    .list button.on { color: var(--accent); }
  `,
})
export class Select {
  readonly value = model.required<string>();
  readonly options = input<SelectOption[]>([]);
  readonly fieldId = input('');
  readonly ariaLabel = input('');
  readonly placeholder = input('');
  readonly disabled = input(false);
  readonly invalid = input(false);

  readonly open = signal(false);
  readonly active = signal(0);

  readonly current = computed(() => this.options().find((o) => o.value === this.value()));

  toggle(): void {
    if (this.open()) {
      this.open.set(false);
      return;
    }
    this.active.set(Math.max(0, this.options().findIndex((o) => o.value === this.value())));
    this.open.set(true);
  }

  pick(option: SelectOption): void {
    this.value.set(option.value);
    this.open.set(false);
  }

  // a select the keyboard cannot drive is worse than the native one it replaced
  onKey(event: KeyboardEvent): void {
    const last = this.options().length - 1;
    switch (event.key) {
      case 'ArrowDown':
      case 'ArrowUp': {
        event.preventDefault();
        if (!this.open()) {
          this.toggle();
          return;
        }
        const step = event.key === 'ArrowDown' ? 1 : -1;
        this.active.update((i) => Math.min(last, Math.max(0, i + step)));
        return;
      }
      case 'Enter':
      case ' ': {
        if (!this.open()) return;
        event.preventDefault();
        const option = this.options()[this.active()];
        if (option) this.pick(option);
        return;
      }
      case 'Escape':
        this.open.set(false);
    }
  }
}
