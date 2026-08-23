import { TestBed } from '@angular/core/testing';

import { TemplateForm } from './template-form';
import { I18n } from '../core/i18n/i18n';
import { Template } from '../core/models';

const minecraft = {
  id: 'minecraft-java',
  fields: [
    { key: 'EULA', label: 'Mojang EULA', type: 'bool', default: 'true', required: true,
      help: 'catalog help for EULA' },
    { key: 'MAX_PLAYERS', label: 'Max players', type: 'int', default: '10' },
    { key: 'VIEW_DISTANCE', label: 'Render distance', type: 'int', default: '10', advanced: true },
  ],
} as Template;

const unknownGame = {
  id: 'some/new-game',
  fields: [{ key: 'SEED', label: 'Seed', type: 'text', help: 'text only the catalog has' }],
} as Template;

describe('TemplateForm', () => {
  function form(template: Template): TemplateForm {
    const fixture = TestBed.createComponent(TemplateForm);
    fixture.componentRef.setInput('template', template);
    fixture.componentRef.setInput('values', {});
    return fixture.componentInstance;
  }

  beforeEach(() => {
    localStorage.removeItem('okdock.locale');
    TestBed.configureTestingModule({});
    TestBed.inject(I18n).setPref('pt');
  });

  afterEach(() => localStorage.removeItem('okdock.locale'));

  it('separates basic fields from advanced ones', () => {
    const f = form(minecraft);

    expect(f.basic().map((x) => x.key)).toEqual(['EULA', 'MAX_PLAYERS']);
    expect(f.advanced().map((x) => x.key)).toEqual(['VIEW_DISTANCE']);
  });

  it('writes the field problem from the code the API sent', () => {
    const fixture = TestBed.createComponent(TemplateForm);
    fixture.componentRef.setInput('template', minecraft);
    fixture.componentRef.setInput('values', {});
    fixture.componentRef.setInput('problems', [
      { field: 'MAX_PLAYERS', code: 'below_min', params: { min: 1 } },
    ]);

    expect(fixture.componentInstance.error(minecraft.fields![1])).toBe('mínimo é 1');
    expect(fixture.componentInstance.error(minecraft.fields![0])).toBeUndefined();
  });

  it('shows the code when the problem comes from a newer API', () => {
    const fixture = TestBed.createComponent(TemplateForm);
    fixture.componentRef.setInput('template', minecraft);
    fixture.componentRef.setInput('values', {});
    fixture.componentRef.setInput('problems', [{ field: 'EULA', code: 'new_rule' }]);

    expect(fixture.componentInstance.error(minecraft.fields![0])).toBe('new_rule');
  });

  it('prefers the translated help over the catalog one', () => {
    const f = form(minecraft);

    expect(f.fieldHelp(minecraft.fields![0])).toBe('A imagem não sobe sem isto aceito.');

    TestBed.inject(I18n).setPref('en');
    expect(f.fieldHelp(minecraft.fields![0])).toBe('The image does not start without this accepted.');
  });

  it('falls back to the catalog help for a game the screen has not translated yet', () => {
    const f = form(unknownGame);
    TestBed.inject(I18n).setPref('en');

    expect(f.fieldHelp(unknownGame.fields![0])).toBe('text only the catalog has');
  });

  it('uses the field default until someone types', () => {
    const f = form(minecraft);

    expect(f.value(minecraft.fields![1])).toBe('10');
    f.set(minecraft.fields![1], '20');
    expect(f.value(minecraft.fields![1])).toBe('20');
  });

  it('stores a boolean as text, which is what goes to the compose', () => {
    const f = form(minecraft);

    f.set(minecraft.fields![0], false);

    expect(f.values()['EULA']).toBe('false');
    expect(f.checked(minecraft.fields![0])).toBeFalse();
  });
});
