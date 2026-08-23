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

  it('renaming adds the new name and only then drops the old one', () => {
    settings.rename('smp', 'novo');

    const add = http.expectOne('/api/v1/dns/domains');
    expect(add.request.method).toBe('POST');
    expect(add.request.body).toEqual({ domain: 'novo' });
    add.flush({ domain: 'novo', hostname: 'novo.duckdns.org' });

    const drop = http.expectOne('/api/v1/dns/domains/smp');
    expect(drop.request.method).toBe('DELETE');
    drop.flush(null);

    http.expectOne('/api/v1/dns').flush(dns);
    expect(settings.busyDomain()).withContext('the row stayed stuck').toBeNull();
  });

  it('a rejected new name does not erase what already worked', () => {
    settings.rename('smp', 'taken');

    http.expectOne('/api/v1/dns/domains').flush(
      { error: 'dns_rejected', message: 'duckdns said KO' },
      { status: 422, statusText: 'Unprocessable Entity' },
    );

    http.expectNone('/api/v1/dns/domains/smp');
    expect(settings.domainError()).toBe(
      'o duckdns recusou: confira se o token está certo e se esse nome é da sua conta',
    );
    http.expectOne('/api/v1/dns').flush(dns);
  });

  it('a new token with names on the list checks every one of them', () => {
    settings.tokenDraft.set('token-novo');

    settings.saveToken();
    const save = http.expectOne((r) => r.method === 'PUT' && r.url === '/api/v1/dns');
    expect(save.request.body).toEqual({ token: 'token-novo' });
    save.flush(dns);
    http.expectOne((r) => r.method === 'GET' && r.url === '/api/v1/dns').flush(dns);
    http.expectOne('/api/v1/dns/sync').flush(null);

    expect(settings.tokenNote()).toBe('token gravado; conferindo os nomes da lista…');
    expect(settings.tokenHidden()).withContext('the token should go back to hidden').toBeTrue();
  });

  it('says who owns a name that is already linked', () => {
    expect(settings.instanceFor('smp')).toBe('smp-familia');
    expect(settings.instanceFor('outro')).toBe('');
  });

  it('only offers to save the root once it changed', () => {
    store.system.set({ root: '/srv/games' } as SystemInfo);

    settings.rootDraft.set('/srv/games');
    expect(settings.rootChanged()).toBeFalse();

    settings.rootDraft.set('/mnt/jogos');
    expect(settings.rootChanged()).toBeTrue();
  });

  it('shows the docker version, or that it did not answer', () => {
    store.system.set({ dockerVersion: '27.1.1' } as SystemInfo);
    expect(settings.dockerLabel()).toBe('versão 27.1.1');

    store.system.set({ dockerError: 'sem daemon' } as SystemInfo);
    expect(settings.dockerLabel()).toBe('não respondeu');
  });
});
