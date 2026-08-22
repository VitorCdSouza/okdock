import { TestBed } from '@angular/core/testing';

import { ProviderForm } from './provider-form';
import { I18n } from '../core/i18n/i18n';
import { Provider } from '../core/models';

const minecraft = {
  id: 'itzg/minecraft-server',
  fields: [
    { key: 'EULA', label: 'EULA da Mojang', type: 'bool', default: 'true', required: true,
      help: 'A imagem não sobe sem isto aceito.' },
    { key: 'MAX_PLAYERS', label: 'Máximo de jogadores', type: 'int', default: '10' },
    { key: 'VIEW_DISTANCE', label: 'Distância de render', type: 'int', default: '10', advanced: true },
  ],
} as Provider;

const desconhecido = {
  id: 'algum/jogo-novo',
  fields: [{ key: 'SEED', label: 'Seed', type: 'text', help: 'texto que só o catálogo tem' }],
} as Provider;

describe('ProviderForm', () => {
  function form(provider: Provider): ProviderForm {
    const fixture = TestBed.createComponent(ProviderForm);
    fixture.componentRef.setInput('provider', provider);
    fixture.componentRef.setInput('values', {});
    return fixture.componentInstance;
  }

  beforeEach(() => {
    localStorage.removeItem('okdock.locale');
    TestBed.configureTestingModule({});
    TestBed.inject(I18n).setPref('pt');
  });

  afterEach(() => localStorage.removeItem('okdock.locale'));

  it('separa campos básicos dos avançados', () => {
    const f = form(minecraft);

    expect(f.basic().map((x) => x.key)).toEqual(['EULA', 'MAX_PLAYERS']);
    expect(f.advanced().map((x) => x.key)).toEqual(['VIEW_DISTANCE']);
  });

  it('escreve o problema do campo pelo código que a API mandou', () => {
    const fixture = TestBed.createComponent(ProviderForm);
    fixture.componentRef.setInput('provider', minecraft);
    fixture.componentRef.setInput('values', {});
    fixture.componentRef.setInput('problems', [
      { field: 'MAX_PLAYERS', code: 'below_min', params: { min: 1 } },
    ]);

    expect(fixture.componentInstance.error(minecraft.fields![1])).toBe('mínimo é 1');
    expect(fixture.componentInstance.error(minecraft.fields![0])).toBeUndefined();
  });

  it('mostra o código quando o problema é de uma versão mais nova da API', () => {
    const fixture = TestBed.createComponent(ProviderForm);
    fixture.componentRef.setInput('provider', minecraft);
    fixture.componentRef.setInput('values', {});
    fixture.componentRef.setInput('problems', [{ field: 'EULA', code: 'regra_nova' }]);

    expect(fixture.componentInstance.error(minecraft.fields![0])).toBe('regra_nova');
  });

  it('prefere a ajuda traduzida à do catálogo', () => {
    const f = form(minecraft);

    expect(f.fieldHelp(minecraft.fields![0])).toBe('A imagem não sobe sem isto aceito.');

    TestBed.inject(I18n).setPref('en');
    expect(f.fieldHelp(minecraft.fields![0])).toBe('The image does not start without this accepted.');
  });

  it('cai na ajuda do catálogo para jogo que a tela ainda não traduziu', () => {
    const f = form(desconhecido);
    TestBed.inject(I18n).setPref('en');

    expect(f.fieldHelp(desconhecido.fields![0])).toBe('texto que só o catálogo tem');
  });

  it('usa o default do campo enquanto ninguém digitou', () => {
    const f = form(minecraft);

    expect(f.value(minecraft.fields![1])).toBe('10');
    f.set(minecraft.fields![1], '20');
    expect(f.value(minecraft.fields![1])).toBe('20');
  });

  it('grava booleano como texto, que é o que vai para o compose', () => {
    const f = form(minecraft);

    f.set(minecraft.fields![0], false);

    expect(f.values()['EULA']).toBe('false');
    expect(f.checked(minecraft.fields![0])).toBeFalse();
  });
});
