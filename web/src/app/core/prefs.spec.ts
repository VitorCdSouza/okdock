import { TestBed } from '@angular/core/testing';

import { Prefs } from './prefs';

const KEY = 'okdock.metrics';

describe('Prefs', () => {
  beforeEach(() => localStorage.removeItem(KEY));
  afterEach(() => localStorage.removeItem(KEY));

  function prefs(): Prefs {
    TestBed.resetTestingModule();
    TestBed.configureTestingModule({});
    return TestBed.inject(Prefs);
  }

  it('starts with every number in the bar', () => {
    expect(prefs().metrics()).toEqual({ cpu: true, memory: true, disk: true, budget: true });
  });

  it('saves what was turned off', () => {
    const p = prefs();

    p.toggle('disk');
    TestBed.tick();

    expect(p.metrics().disk).toBeFalse();
    expect(JSON.parse(localStorage.getItem(KEY)!).disk).toBeFalse();
  });

  it('fills in the defaults the storage does not carry', () => {
    localStorage.setItem(KEY, JSON.stringify({ cpu: false }));

    const p = prefs();

    expect(p.metrics().cpu).withContext('escolha gravada perdida').toBeFalse();
    expect(p.metrics().budget).withContext('a missing key should fall back to the default').toBeTrue();
  });

  it('ignores corrupted storage instead of breaking the screen', () => {
    localStorage.setItem(KEY, 'this is not json');

    expect(prefs().metrics().cpu).toBeTrue();
  });
});
