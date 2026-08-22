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

  it('traduz a mesma chave nos dois idiomas', () => {
    expect(i18n.t('common.save')).toBe('Salvar');

    i18n.setPref('en');

    expect(i18n.t('common.save')).toBe('Save');
  });

  it('preenche os parâmetros da mensagem', () => {
    expect(i18n.t('app.created', { name: 'smp' })).toBe('smp foi criada');
  });

  it('escolhe singular ou plural pelo número', () => {
    expect(i18n.plural('app.dns.names', 1)).toBe('duckdns 1 nome');
    expect(i18n.plural('app.dns.names', 3)).toBe('duckdns 3 nomes');
  });

  it('grava a preferência para a próxima sessão', () => {
    i18n.setPref('en');
    TestBed.tick();

    expect(localStorage.getItem('okdock.locale')).toBe('en');
  });

  it('tem as mesmas chaves nas duas tabelas', () => {
    expect(Object.keys(en).sort()).toEqual(Object.keys(pt).sort());
  });
});
