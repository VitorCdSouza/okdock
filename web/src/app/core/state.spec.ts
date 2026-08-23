import { TestBed } from '@angular/core/testing';
import { provideHttpClient } from '@angular/common/http';
import { provideHttpClientTesting, HttpTestingController } from '@angular/common/http/testing';
import { Subject } from 'rxjs';

import { Store } from './state';
import { Events } from './events';
import { I18n } from './i18n/i18n';
import { Instance, ServerEvent, Template } from './models';

function instance(over: Partial<Instance> = {}): Instance {
  return {
    name: 'smp',
    templateId: 'minecraft-java',
    category: 'games',
    image: 'itzg/minecraft-server:java21',
    env: {},
    ports: [{ host: 25565, container: 25565, protocol: 'tcp' }],
    mounts: [],
    memoryLimit: '4g',
    cpus: 2,
    restart: 'unless-stopped',
    stopGraceSeconds: 120,
    createdAt: '2026-08-21T00:00:00Z',
    updatedAt: '2026-08-21T00:00:00Z',
    dir: '/srv/games/smp',
    state: 'running',
    ...over,
  };
}

describe('Store', () => {
  let store: Store;
  let events: Subject<ServerEvent>;
  let http: HttpTestingController;

  beforeEach(() => {
    localStorage.removeItem('okdock.locale');
    events = new Subject<ServerEvent>();
    TestBed.configureTestingModule({
      providers: [
        provideHttpClient(),
        provideHttpClientTesting(),
        { provide: Events, useValue: { stream: () => events.asObservable() } },
      ],
    });
    store = TestBed.inject(Store);
    http = TestBed.inject(HttpTestingController);
    TestBed.inject(I18n).setPref('pt');
  });

  afterEach(() => localStorage.removeItem('okdock.locale'));

  it('filters by name, port and image', () => {
    store.instances.set([instance(), instance({ name: 'terra', templateId: 'terraria-tshock', ports: [] })]);

    store.search.set('25565');
    expect(store.filtered().map((i) => i.name)).toEqual(['smp']);

    store.search.set('TERRA');
    expect(store.filtered().map((i) => i.name)).withContext('the search is case-insensitive').toEqual(['terra']);
  });

  it('the category filter and the text filter apply together', () => {
    store.instances.set([instance(), instance({ name: 'outro' })]);
    store.categoryFilter.set('games');
    store.search.set('outro');

    expect(store.filtered().map((i) => i.name)).toEqual(['outro']);
  });

  it('counts by category, in the order the API sent', () => {
    store.categories.set(['games', 'media', 'other']);
    store.instances.set([
      instance(),
      instance({ name: 'dois' }),
      instance({ name: 'filmes', templateId: 'jellyfin', category: 'media' }),
    ]);

    expect(store.categoryCounts()).toEqual([
      { category: 'games', count: 2 },
      { category: 'media', count: 1 },
    ]);
  });

  it('groups templates by category, skipping the ones nobody uses', () => {
    store.categories.set(['games', 'media', 'other']);
    store.templates.set([
      { id: 'minecraft-java', category: 'games' } as Template,
      { id: 'jellyfin', category: 'media' } as Template,
      { id: 'terraria-tshock', category: 'games' } as Template,
    ]);

    expect(store.byCategory().map((g) => [g.category, g.templates.map((t) => t.id)])).toEqual([
      ['games', ['minecraft-java', 'terraria-tshock']],
      ['media', ['jellyfin']],
    ]);
  });

  it('puts provisioning and starting in the running column', () => {
    store.instances.set([
      instance({ name: 'a', state: 'starting' }),
      instance({ name: 'b', state: 'provisioning' }),
      instance({ name: 'c', state: 'running' }),
      instance({ name: 'd', state: 'stopped' }),
    ]);

    expect(store.byColumn('running').map((i) => i.name)).toEqual(['a', 'b', 'c']);
    expect(store.byColumn('stopped').map((i) => i.name)).toEqual(['d']);
  });

  it('builds the end-of-operation notice from the event type', () => {
    store.start();
    http.expectOne('/api/v1/templates').flush({ templates: [], categories: [] });
    http.expectOne('/api/v1/instances').flush({ instances: [], states: [] });
    http.expectOne('/api/v1/system').flush({});
    http.expectOne('/api/v1/dns').flush({ token: '', suffix: '.duckdns.org', links: [], domains: [] });

    events.next({ type: 'instance.updated', instance: 'smp', message: 'texto da API' });

    expect(store.toast()).toBe('smp foi atualizada; o mundo nos volumes foi preservado');
  });
});
