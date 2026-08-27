import {
  ChangeDetectionStrategy,
  Component,
  ElementRef,
  computed,
  effect,
  input,
  model,
  signal,
  viewChild,
} from '@angular/core';

export interface SelectOption {
  value: string;
  label: string;
}

// where the list is drawn, in viewport coordinates
interface Box {
  left: number;
  width: number;
  maxHeight: number;
  top?: number;
  bottom?: number;
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
      <button #trigger type="button" class="trigger mono" role="combobox"
              [attr.id]="fieldId() || null" [attr.aria-expanded]="open()"
              [attr.aria-label]="ariaLabel() || null" [disabled]="disabled()"
              [class.invalid]="invalid()" (click)="toggle()" (keydown)="onKey($event)">
        <span class="label" [class.none]="!current()">{{ current()?.label ?? placeholder() }}</span>
        <svg class="caret" viewBox="0 0 10 6" aria-hidden="true">
          <path d="M1 1l4 4 4-4" />
        </svg>
      </button>

      @if (open()) {
        <ul #list class="list" role="listbox" popover="manual"
            [style.left.px]="box().left" [style.width.px]="box().width"
            [style.top.px]="box().top" [style.bottom.px]="box().bottom"
            [style.max-height.px]="box().maxHeight">
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
      height: var(--field-h);
      padding: 0 9px;
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

    /* the top layer is what keeps the list out of the overflow of a dialog */
    .list {
      position: fixed;
      inset: auto;
      z-index: 60;
      display: block;
      overflow-y: auto;
      overscroll-behavior: contain;
      margin: 0;
      padding: 4px;
      list-style: none;
      color: var(--fg-soft);
      background: var(--bg-header);
      border: 1px solid var(--line-strong);
      border-radius: var(--r-sm);
      box-shadow: 0 8px 24px rgba(0, 0, 0, .5);
      --scroll-bg: var(--bg-header);
      scrollbar-color: var(--line-strong) var(--bg-header);
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
  readonly box = signal<Box>({ left: 0, width: 0, maxHeight: 260 });

  readonly current = computed(() => this.options().find((o) => o.value === this.value()));

  private readonly trigger = viewChild<ElementRef<HTMLElement>>('trigger');
  private readonly list = viewChild<ElementRef<HTMLElement>>('list');

  constructor() {
    effect((onCleanup) => {
      const list = this.list()?.nativeElement;
      if (!list) return;
      this.place();
      if (!list.matches(':popover-open')) list.showPopover?.();
      // the list no longer moves with the field, so a scroll has to move it
      const follow = () => this.place();
      window.addEventListener('scroll', follow, true);
      window.addEventListener('resize', follow);
      onCleanup(() => {
        window.removeEventListener('scroll', follow, true);
        window.removeEventListener('resize', follow);
      });
    });
  }

  toggle(): void {
    if (this.open()) {
      this.open.set(false);
      return;
    }
    this.active.set(Math.max(0, this.options().findIndex((o) => o.value === this.value())));
    this.place();
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

  // opens downwards, unless what is left under the field is the smaller half
  private place(): void {
    const trigger = this.trigger()?.nativeElement;
    if (!trigger) return;
    const rect = trigger.getBoundingClientRect();
    const gap = 4;
    const edge = 12;
    const below = window.innerHeight - rect.bottom - gap - edge;
    const above = rect.top - gap - edge;
    const up = below < Math.min(200, above);
    const room = Math.max(120, up ? above : below);
    const left = Math.max(0, Math.min(rect.left, window.innerWidth - rect.width - edge));
    this.box.set({
      left,
      width: rect.width,
      maxHeight: Math.min(260, room),
      top: up ? undefined : rect.bottom + gap,
      bottom: up ? window.innerHeight - rect.top + gap : undefined,
    });
  }
}
