import { TestBed } from '@angular/core/testing';
import { provideHttpClient } from '@angular/common/http';
import { provideHttpClientTesting, HttpTestingController } from '@angular/common/http/testing';

import { Settings } from './settings';
import { Store } from '../../core/state';
import { I18n } from '../../core/i18n/i18n';
import { DnsStatus, SystemInfo } from '../../core/models';

const dns: DnsStatus = {
  token: 'token-velho',
  suffix: '.duckdns.org',
  links: [{ domain: 'smp', hostname: 'smp.duckdns.org', instance: 'smp-familia' }],
  domains: [{ domain: 'smp', hostname: 'smp.duckdns.org', lastIp: '187.12.3.4' }],
};

describe('Settings', () => {
  let settings: Settings;
  let store: Store;
  let http: HttpTestingController;

  beforeEach(() => {
    localStorage.removeItem('okdock.locale');
    TestBed.configureTestingModule({
      providers: [provideHttpClient(), provideHttpClientTesting()],
    });
    settings = TestBed.createComponent(Settings).componentInstance;
    store = TestBed.inject(Store);
    http = TestBed.inject(HttpTestingController);
    TestBed.inject(I18n).setPref('pt');
    store.dns.set(dns);
  });

  afterEach(() => localStorage.removeItem('okdock.locale'));

  it('renomear é cadastrar o novo e só então soltar o antigo', () => {
    settings.rename('smp', 'novo');

    const add = http.expectOne('/api/v1/dns/domains');
    expect(add.request.method).toBe('POST');
    expect(add.request.body).toEqual({ domain: 'novo' });
    add.flush({ domain: 'novo', hostname: 'novo.duckdns.org' });

    const drop = http.expectOne('/api/v1/dns/domains/smp');
    expect(drop.request.method).toBe('DELETE');
    drop.flush(null);

    http.expectOne('/api/v1/dns').flush(dns);
    expect(settings.busyDomain()).withContext('a linha ficou travada').toBeNull();
  });

  it('nome novo recusado não apaga o que já funcionava', () => {
    settings.rename('smp', 'ocupado');

    http.expectOne('/api/v1/dns/domains').flush(
      { error: 'dns_rejected', message: 'o duckdns recusou' },
      { status: 422, statusText: 'Unprocessable Entity' },
    );

    http.expectNone('/api/v1/dns/domains/smp');
    expect(settings.domainError()).toBe(
      'o duckdns recusou: confira se o token está certo e se esse nome é da sua conta',
    );
    http.expectOne('/api/v1/dns').flush(dns);
  });

  it('token novo com nomes na lista manda conferir todos', () => {
    settings.tokenDraft.set('token-novo');

    settings.saveToken();
    const save = http.expectOne((r) => r.method === 'PUT' && r.url === '/api/v1/dns');
    expect(save.request.body).toEqual({ token: 'token-novo' });
    save.flush(dns);
    http.expectOne((r) => r.method === 'GET' && r.url === '/api/v1/dns').flush(dns);
    http.expectOne('/api/v1/dns/sync').flush(null);

    expect(settings.tokenNote()).toBe('token gravado; conferindo os nomes da lista…');
    expect(settings.tokenHidden()).withContext('o token devia voltar a ficar escondido').toBeTrue();
  });

  it('diz de quem é o nome já vinculado', () => {
    expect(settings.instanceFor('smp')).toBe('smp-familia');
    expect(settings.instanceFor('outro')).toBe('');
  });

  it('só oferece salvar a raiz quando ela mudou', () => {
    store.system.set({ root: '/srv/games' } as SystemInfo);

    settings.rootDraft.set('/srv/games');
    expect(settings.rootChanged()).toBeFalse();

    settings.rootDraft.set('/mnt/jogos');
    expect(settings.rootChanged()).toBeTrue();
  });

  it('mostra a versão do docker, ou que ele não respondeu', () => {
    store.system.set({ dockerVersion: '27.1.1' } as SystemInfo);
    expect(settings.dockerLabel()).toBe('versão 27.1.1');

    store.system.set({ dockerError: 'sem daemon' } as SystemInfo);
    expect(settings.dockerLabel()).toBe('não respondeu');
  });
});
