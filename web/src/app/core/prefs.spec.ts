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

  it('começa com todos os números na barra', () => {
    expect(prefs().metrics()).toEqual({ cpu: true, memory: true, disk: true, budget: true });
  });

  it('grava o que foi desligado', () => {
    const p = prefs();

    p.toggle('disk');
    TestBed.tick();

    expect(p.metrics().disk).toBeFalse();
    expect(JSON.parse(localStorage.getItem(KEY)!).disk).toBeFalse();
  });

  it('completa com os padrões o que o storage não traz', () => {
    localStorage.setItem(KEY, JSON.stringify({ cpu: false }));

    const p = prefs();

    expect(p.metrics().cpu).withContext('escolha gravada perdida').toBeFalse();
    expect(p.metrics().budget).withContext('chave ausente devia cair no padrão').toBeTrue();
  });

  it('ignora storage corrompido em vez de quebrar a tela', () => {
    localStorage.setItem(KEY, 'isto não é json');

    expect(prefs().metrics().cpu).toBeTrue();
  });
});
