import { TestBed } from '@angular/core/testing';

import { Select } from './select';

describe('Select', () => {
  function make(value = 'media') {
    const fixture = TestBed.createComponent(Select);
    fixture.componentRef.setInput('value', value);
    fixture.componentRef.setInput('options', [
      { value: 'games', label: 'Games' },
      { value: 'media', label: 'Media' },
      { value: 'other', label: 'Other' },
    ]);
    fixture.detectChanges();
    return fixture;
  }

  beforeEach(() => TestBed.configureTestingModule({}));

  it('shows the label of what is chosen, not the value', () => {
    const fixture = make();

    expect(fixture.nativeElement.querySelector('.trigger .label').textContent.trim()).toBe('Media');
    expect(fixture.nativeElement.querySelector('.list')).toBeNull();
  });

  it('opens on the chosen option, so the arrows start from it', () => {
    const fixture = make();
    fixture.componentInstance.toggle();
    fixture.detectChanges();

    expect(fixture.componentInstance.active()).toBe(1);
    const options: HTMLElement[] = Array.from(fixture.nativeElement.querySelectorAll('.list button'));
    expect(options.map((o) => o.textContent!.trim())).toEqual(['Games', 'Media', 'Other']);
    expect(options[1].classList).toContain('on');
  });

  it('picking closes it and reports the value', () => {
    const fixture = make();
    const c = fixture.componentInstance;
    c.toggle();

    c.pick({ value: 'other', label: 'Other' });

    expect(c.value()).toBe('other');
    expect(c.open()).toBeFalse();
  });

  it('the keyboard drives it, since it replaced a native select', () => {
    const fixture = make('games');
    const c = fixture.componentInstance;

    c.onKey(new KeyboardEvent('keydown', { key: 'ArrowDown' }));
    expect(c.open()).toBeTrue();
    expect(c.active()).toBe(0);

    c.onKey(new KeyboardEvent('keydown', { key: 'ArrowDown' }));
    c.onKey(new KeyboardEvent('keydown', { key: 'Enter' }));

    expect(c.value()).toBe('media');
    expect(c.open()).toBeFalse();

    // and it does not walk past either end
    c.toggle();
    for (let i = 0; i < 5; i++) c.onKey(new KeyboardEvent('keydown', { key: 'ArrowUp' }));
    expect(c.active()).toBe(0);
  });

  it('escape closes without choosing', () => {
    const fixture = make();
    const c = fixture.componentInstance;
    c.toggle();

    c.onKey(new KeyboardEvent('keydown', { key: 'Escape' }));

    expect(c.open()).toBeFalse();
    expect(c.value()).toBe('media');
  });
});
