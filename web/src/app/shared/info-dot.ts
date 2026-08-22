import {
  ChangeDetectionStrategy,
  Component,
  ElementRef,
  inject,
  input,
  viewChild,
} from '@angular/core';

@Component({
  selector: 'gd-info',
  changeDetection: ChangeDetectionStrategy.OnPush,
  host: {
    'tabindex': '0',
    'role': 'note',
    '[attr.aria-label]': 'text()',
    '(mouseenter)': 'open()',
    '(mouseleave)': 'scheduleClose()',
    '(focus)': 'open()',
    '(blur)': 'close()',
    '(keydown.escape)': 'close()',
  },
  styles: `
    :host {
      position: relative;
      display: inline-grid;
      place-items: center;
      width: 13px; height: 13px;
      flex: none;
      border: 1px solid var(--line-hover);
      border-radius: 50%;
      font: 600 9px/1 var(--sans);
      color: var(--fg-dim);
      cursor: default;
      align-self: center;
    }
    :host::before { content: 'i'; }
    :host:hover, :host:focus-visible { color: var(--accent); border-color: var(--accent-line); outline: none; }

    .tip {
      position: fixed;
      margin: 0;
      inset: auto;
      width: max-content;
      max-width: min(260px, calc(100vw - 24px));
      padding: 7px 9px;
      background: var(--bg-header);
      border: 1px solid var(--line-strong);
      border-radius: var(--r-sm);
      box-shadow: 0 8px 22px rgba(0, 0, 0, .5);
      font: 400 var(--fs-xs)/1.5 var(--sans);
      letter-spacing: 0;
      text-align: left;
      white-space: pre-line;
      color: var(--fg-soft);
      cursor: text;
      user-select: text;
      overflow: visible;
    }
    .tip:not(.open) { display: none; }
  `,
  template: `<span #tip class="tip" popover="manual"
                   (mouseenter)="cancelClose()" (mouseleave)="scheduleClose()">{{ text() }}</span>`,
})
export class InfoDot {
  readonly text = input.required<string>();

  private readonly host = inject(ElementRef<HTMLElement>);
  private readonly tip = viewChild.required<ElementRef<HTMLElement>>('tip');
  private closeTimer?: ReturnType<typeof setTimeout>;

  open(): void {
    this.cancelClose();
    const el = this.tip().nativeElement;
    el.classList.add('open');
    try {
      el.showPopover();
    } catch {
    }
    this.place();
  }

  close(): void {
    this.cancelClose();
    const el = this.tip().nativeElement;
    el.classList.remove('open');
    try {
      el.hidePopover();
    } catch {
    }
  }

  scheduleClose(): void {
    this.cancelClose();
    this.closeTimer = setTimeout(() => this.close(), 180);
  }

  cancelClose(): void {
    clearTimeout(this.closeTimer);
  }

  private place(): void {
    const el = this.tip().nativeElement;
    const anchor = (this.host.nativeElement as HTMLElement).getBoundingClientRect();
    const { offsetWidth: w, offsetHeight: h } = el;
    const margin = 8;

    const left = Math.min(
      Math.max(margin, anchor.right + 4 - w),
      Math.max(margin, window.innerWidth - w - margin),
    );
    const below = anchor.bottom + 6;
    const top = below + h > window.innerHeight - margin
      ? Math.max(margin, anchor.top - 6 - h)
      : below;

    el.style.left = `${left}px`;
    el.style.top = `${top}px`;
  }
}
