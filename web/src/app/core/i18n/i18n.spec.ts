import { TestBed } from '@angular/core/testing';

import { I18n } from './i18n';
import { pt } from './messages.pt';
import { en } from './messages.en';

describe('I18n', () => {
  let i18n: I18n;

  beforeEach(() => {
    localStorage.removeItem('okdock.locale');
    TestBed.configureTestingModule({});
    i18n = TestBed.inject(I18n);
    i18n.setPref('pt');
  });

  afterEach(() => localStorage.removeItem('okdock.locale'));

  it('translates the same key in both languages', () => {
    expect(i18n.t('common.save')).toBe('Salvar');

    i18n.setPref('en');

    expect(i18n.t('common.save')).toBe('Save');
  });

  it('fills in the message parameters', () => {
    expect(i18n.t('app.created', { name: 'smp' })).toBe('smp foi criada');
  });

  it('picks singular or plural by the number', () => {
    expect(i18n.plural('app.dns.names', 1)).toBe('duckdns 1 nome');
    expect(i18n.plural('app.dns.names', 3)).toBe('duckdns 3 nomes');
  });

  it('saves the preference for the next session', () => {
    i18n.setPref('en');
    TestBed.tick();

    expect(localStorage.getItem('okdock.locale')).toBe('en');
  });

  it('has the same keys in both tables', () => {
    expect(Object.keys(en).sort()).toEqual(Object.keys(pt).sort());
  });
});
