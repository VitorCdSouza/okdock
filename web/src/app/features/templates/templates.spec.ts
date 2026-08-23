import { TestBed } from '@angular/core/testing';
import { provideHttpClient } from '@angular/common/http';
import { provideHttpClientTesting, HttpTestingController } from '@angular/common/http/testing';

import { Templates } from './templates';
import { Store } from '../../core/state';
import { Template } from '../../core/models';

function template(over: Partial<Template> = {}): Template {
  return {
    id: 'minecraft-java',
    name: 'Minecraft (Java)',
    category: 'games',
    short: 'MC',
    image: 'itzg/minecraft-server:java21',
    description: '',
    ports: [],
    volumes: [{ host: './data', container: '/data', data: true }],
    defaultMemory: '4g',
    minMemory: '2g',
    defaultCpus: 2,
    stopGraceSeconds: 120,
    fields: [],
    builtin: true,
    ...over,
  };
}

describe('Templates — cadastro de template', () => {
  let screen: Templates;
  let store: Store;
  let http: HttpTestingController;

  beforeEach(() => {
    localStorage.removeItem('okdock.locale');
    TestBed.configureTestingModule({
      providers: [provideHttpClient(), provideHttpClientTesting()],
    });
    screen = TestBed.createComponent(Templates).componentInstance;
    store = TestBed.inject(Store);
    http = TestBed.inject(HttpTestingController);
  });

  afterEach(() => {
    localStorage.removeItem('okdock.locale');
    http.verify();
  });

  it('template novo vai por POST e recarrega o catálogo', () => {
    screen.create();
    screen.patch({ id: 'jellyfin', name: 'Jellyfin', category: 'media', image: 'jellyfin/jellyfin:10.9' });
    screen.save();

    const req = http.expectOne('/api/v1/templates');
    expect(req.request.method).toBe('POST');
    expect(req.request.body.id).toBe('jellyfin');
    req.flush(template({ id: 'jellyfin', name: 'Jellyfin', category: 'media', builtin: false }));

    expect(screen.isNew()).withContext('depois de gravar deixa de ser novo').toBeFalse();
    http.expectOne('/api/v1/templates').flush({ templates: [], categories: [] });
  });

  it('editar template existente vai por PUT no id dele', () => {
    screen.edit(template());
    screen.patch({ defaultMemory: '8g' });
    screen.save();

    const req = http.expectOne('/api/v1/templates/minecraft-java');
    expect(req.request.method).toBe('PUT');
    expect(req.request.body.defaultMemory).toBe('8g');
    req.flush(template({ defaultMemory: '8g', builtin: false }));

    http.expectOne('/api/v1/templates').flush({ templates: [], categories: [] });
  });

  it('editar não mexe no template do store antes de gravar', () => {
    const original = template();
    store.templates.set([original]);

    screen.edit(original);
    screen.patch({ name: 'Outro nome' });

    expect(store.templates()[0].name).toBe('Minecraft (Java)');
  });

  it('opções do enum entram como texto valor=rótulo', () => {
    screen.create();
    screen.addField();
    screen.patchField(0, { key: 'MODE', type: 'enum' });
    screen.setOptions(0, 'survival=Survival, creative');

    expect(screen.draft()!.fields![0].options).toEqual([
      { value: 'survival', label: 'Survival' },
      { value: 'creative', label: 'creative' },
    ]);
  });

  it('erro de validação da API fica na tela e o rascunho continua aberto', () => {
    screen.create();
    screen.patch({ id: 'x y', name: '' });
    screen.save();

    http.expectOne('/api/v1/templates').flush(
      { error: 'invalid_fields', message: 'inválido', problems: [{ field: 'id', code: 'bad_template_id' }] },
      { status: 422, statusText: 'Unprocessable Entity' },
    );

    expect(screen.error()?.problems.length).toBe(1);
    expect(screen.draft()).withContext('o rascunho não pode se perder no erro').not.toBeNull();
  });

  it('apagar a edição de um template de fábrica chama DELETE e fecha o editor', () => {
    screen.edit(template({ builtin: false }));
    screen.remove();

    const req = http.expectOne('/api/v1/templates/minecraft-java');
    expect(req.request.method).toBe('DELETE');
    req.flush(null, { status: 204, statusText: 'No Content' });

    expect(screen.draft()).toBeNull();
    http.expectOne('/api/v1/templates').flush({ templates: [], categories: [] });
  });
});
