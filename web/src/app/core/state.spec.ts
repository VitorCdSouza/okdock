import { TestBed } from '@angular/core/testing';
import { provideHttpClient } from '@angular/common/http';
import { provideHttpClientTesting, HttpTestingController } from '@angular/common/http/testing';
import { Subject } from 'rxjs';

import { Store } from './state';
import { Events } from './events';
import { I18n } from './i18n/i18n';
import { Instance, Provider, ServerEvent } from './models';

function instance(over: Partial<Instance> = {}): Instance {
  return {
    name: 'smp',
    providerId: 'itzg/minecraft-server',
    game: 'minecraft-java',
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
    localStorage.removeItem('gamedock.locale');
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

  afterEach(() => localStorage.removeItem('gamedock.locale'));

  it('filtra por nome, porta e imagem', () => {
    store.instances.set([instance(), instance({ name: 'terra', game: 'terraria', ports: [] })]);

    store.search.set('25565');
    expect(store.filtered().map((i) => i.name)).toEqual(['smp']);

    store.search.set('TERRA');
    expect(store.filtered().map((i) => i.name)).withContext('busca é sem caixa').toEqual(['terra']);
  });

  it('o filtro de jogo e o de texto valem juntos', () => {
    store.instances.set([instance(), instance({ name: 'outro' })]);
    store.gameFilter.set('minecraft-java');
    store.search.set('outro');

    expect(store.filtered().map((i) => i.name)).toEqual(['outro']);
  });

  it('conta por jogo usando o rótulo do provedor', () => {
    store.providers.set([
      { id: 'itzg/minecraft-server', game: 'minecraft-java', gameLabel: 'Minecraft (Java)' } as Provider,
    ]);
    store.instances.set([instance(), instance({ name: 'dois' })]);

    expect(store.gameCounts()).toEqual([
      { game: 'minecraft-java', label: 'Minecraft (Java)', count: 2 },
    ]);
  });

  it('cai no nome cru do jogo quando o provedor sumiu do catálogo', () => {
    store.instances.set([instance({ providerId: 'foi-embora', game: 'valheim' })]);

    expect(store.gameCounts()[0].label).toBe('valheim');
  });

  it('põe provisionando e iniciando na coluna de rodando', () => {
    store.instances.set([
      instance({ name: 'a', state: 'starting' }),
      instance({ name: 'b', state: 'provisioning' }),
      instance({ name: 'c', state: 'running' }),
      instance({ name: 'd', state: 'stopped' }),
    ]);

    expect(store.byColumn('running').map((i) => i.name)).toEqual(['a', 'b', 'c']);
    expect(store.byColumn('stopped').map((i) => i.name)).toEqual(['d']);
  });

  it('monta o aviso de fim de operação pelo tipo do evento', () => {
    store.start();
    http.expectOne('/api/v1/providers').flush([]);
    http.expectOne('/api/v1/instances').flush({ instances: [], states: [] });
    http.expectOne('/api/v1/system').flush({});
    http.expectOne('/api/v1/dns').flush({ token: '', suffix: '.duckdns.org', links: [], domains: [] });

    events.next({ type: 'instance.updated', instance: 'smp', message: 'texto da API' });

    expect(store.toast()).toBe('smp foi atualizada; o mundo nos volumes foi preservado');
  });
});
