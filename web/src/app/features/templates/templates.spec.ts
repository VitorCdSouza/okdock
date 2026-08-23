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

describe('Templates: registering a template', () => {
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

  it('a new template goes by POST and reloads the catalog', () => {
    screen.create();
    screen.patch({ id: 'jellyfin', name: 'Jellyfin', category: 'media', image: 'jellyfin/jellyfin:10.9' });
    screen.save();

    const req = http.expectOne('/api/v1/templates');
    expect(req.request.method).toBe('POST');
    expect(req.request.body.id).toBe('jellyfin');
    req.flush(template({ id: 'jellyfin', name: 'Jellyfin', category: 'media', builtin: false }));

    expect(screen.isNew()).withContext('after saving it is no longer new').toBeFalse();
    http.expectOne('/api/v1/templates').flush({ templates: [], categories: [] });
  });

  it('editing an existing template goes by PUT on its id', () => {
    screen.edit(template());
    screen.patch({ defaultMemory: '8g' });
    screen.save();

    const req = http.expectOne('/api/v1/templates/minecraft-java');
    expect(req.request.method).toBe('PUT');
    expect(req.request.body.defaultMemory).toBe('8g');
    req.flush(template({ defaultMemory: '8g', builtin: false }));

    http.expectOne('/api/v1/templates').flush({ templates: [], categories: [] });
  });

  it('editing does not touch the store template before saving', () => {
    const original = template();
    store.templates.set([original]);

    screen.edit(original);
    screen.patch({ name: 'Outro nome' });

    expect(store.templates()[0].name).toBe('Minecraft (Java)');
  });

  it('enum options come in as value=label text', () => {
    screen.create();
    screen.addField();
    screen.patchField(0, { key: 'MODE', type: 'enum' });
    screen.setOptions(0, 'survival=Survival, creative');

    expect(screen.draft()!.fields![0].options).toEqual([
      { value: 'survival', label: 'Survival' },
      { value: 'creative', label: 'creative' },
    ]);
  });

  it('an API validation error stays on screen and the draft stays open', () => {
    screen.create();
    screen.patch({ id: 'x y', name: '' });
    screen.save();

    http.expectOne('/api/v1/templates').flush(
      { error: 'invalid_fields', message: 'invalid', problems: [{ field: 'id', code: 'bad_template_id' }] },
      { status: 422, statusText: 'Unprocessable Entity' },
    );

    expect(screen.error()?.problems.length).toBe(1);
    expect(screen.draft()).withContext('the draft must not be lost on the error').not.toBeNull();
  });

  it('the custom image does not show up in the list', () => {
    store.categories.set(['games', 'other']);
    store.templates.set([template(), template({ id: 'custom', name: 'Imagem custom', category: 'other' })]);

    screen.select('other');
    expect(screen.shown()).toEqual([]);
    expect(screen.count('other')).toBe(0);
  });

  it('the tab shows only the templates of its category', () => {
    store.categories.set(['games', 'media', 'other']);
    store.templates.set([template(), template({ id: 'jellyfin', category: 'media' })]);

    expect(screen.active()).withContext('with nothing picked it opens on the first tab').toBe('games');
    expect(screen.shown().map((t) => t.id)).toEqual(['minecraft-java']);

    screen.select('media');
    expect(screen.shown().map((t) => t.id)).toEqual(['jellyfin']);
    expect(screen.count('media')).toBe(1);
  });

  it('a category typed by hand becomes a tab and the draft is born in it', () => {
    store.categories.set(['games', 'other']);

    screen.startCategory();
    screen.categoryDraft.set('Streaming de Áudio');
    screen.confirmCategory();

    expect(screen.categories()).toContain('streaming-de-audio');
    expect(screen.active()).toBe('streaming-de-audio');
    expect(screen.draft()!.category).toBe('streaming-de-audio');
    expect(screen.adding()).toBeFalse();
  });

  it('a name with no letter or digit does not become a category', () => {
    store.categories.set(['games', 'other']);

    screen.startCategory();
    screen.categoryDraft.set('!!');
    screen.confirmCategory();

    expect(screen.categories()).toEqual(['games', 'other']);
    expect(screen.adding()).withContext('the field stays open to be fixed').toBeTrue();
  });

  it('deleting the edit of a builtin template calls DELETE and closes the editor', () => {
    screen.edit(template({ builtin: false }));
    screen.remove();

    const req = http.expectOne('/api/v1/templates/minecraft-java');
    expect(req.request.method).toBe('DELETE');
    req.flush(null, { status: 204, statusText: 'No Content' });

    expect(screen.draft()).toBeNull();
    http.expectOne('/api/v1/templates').flush({ templates: [], categories: [] });
  });
});
