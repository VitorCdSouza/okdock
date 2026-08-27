import { TestBed } from '@angular/core/testing';
import { provideHttpClient } from '@angular/common/http';
import { provideHttpClientTesting, HttpTestingController } from '@angular/common/http/testing';

import { Settings } from './settings';
import { Store } from '../../core/state';
import { I18n } from '../../core/i18n/i18n';
import { Prefs } from '../../core/prefs';
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
    localStorage.removeItem('okdock.metrics');
    TestBed.configureTestingModule({
      providers: [provideHttpClient(), provideHttpClientTesting()],
    });
    settings = TestBed.createComponent(Settings).componentInstance;
    store = TestBed.inject(Store);
    http = TestBed.inject(HttpTestingController);
    TestBed.inject(I18n).setPref('pt');
    store.dns.set(dns);
  });

  afterEach(() => {
    localStorage.removeItem('okdock.locale');
    localStorage.removeItem('okdock.metrics');
  });

  it('renaming adds the new name and only then drops the old one', () => {
    settings.setName(0, 'novo');
    settings.save();

    const add = http.expectOne('/api/v1/dns/domains');
    expect(add.request.method).toBe('POST');
    expect(add.request.body).toEqual({ domain: 'novo' });
    add.flush({ domain: 'novo', hostname: 'novo.duckdns.org' });

    const drop = http.expectOne('/api/v1/dns/domains/smp');
    expect(drop.request.method).toBe('DELETE');
    drop.flush(null);

    http.expectOne('/api/v1/dns').flush(dns);
    expect(settings.busy()).withContext('the screen stayed stuck').toBeFalse();
  });

  it('a rejected new name does not erase what already worked', () => {
    settings.setName(0, 'taken');
    settings.save();

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

  it('a token saved in its own dialog is checked against the names on the list', () => {
    settings.openToken();
    settings.tokenDraft.set('token-novo');

    settings.saveToken();
    const save = http.expectOne((r) => r.method === 'PUT' && r.url === '/api/v1/dns');
    expect(save.request.body).toEqual({ token: 'token-novo' });
    save.flush({ ...dns, token: 'token-novo' });

    expect(settings.tokenOpen()).withContext('the dialog should have closed').toBeFalse();
    expect(settings.tokenNote()).toBe('token gravado; conferindo os nomes da lista…');

    http.expectOne('/api/v1/dns/sync').flush(null);
    http.expectOne((r) => r.method === 'GET' && r.url === '/api/v1/dns').flush({
      ...dns,
      domains: [{ domain: 'smp', hostname: 'smp.duckdns.org', lastError: 'duckdns said KO' }],
    });

    expect(settings.tokenNote()).toBeNull();
    expect(settings.refused().map((d) => d.hostname)).toEqual(['smp.duckdns.org']);
  });

  it('a token with no name on the list has nothing to be checked against', () => {
    store.dns.set({ ...dns, links: [], domains: [] });
    settings.openToken();
    settings.tokenDraft.set('token-novo');

    settings.saveToken();
    http.expectOne((r) => r.method === 'PUT' && r.url === '/api/v1/dns').flush({
      ...dns,
      token: 'token-novo',
      links: [],
      domains: [],
    });

    http.expectNone('/api/v1/dns/sync');
    expect(settings.tokenNote()).toBe('token gravado; será conferido no primeiro nome da lista');
  });

  it('says who owns a name that is already linked', () => {
    expect(settings.instanceFor('smp')).toBe('smp-familia');
    expect(settings.instanceFor('outro')).toBe('');
  });

  it('only offers to save the root once it changed', () => {
    store.system.set({ root: '/containers' } as SystemInfo);

    settings.rootDraft.set('/containers');
    expect(settings.rootChanged()).toBeFalse();

    settings.rootDraft.set('/mnt/jogos');
    expect(settings.rootChanged()).toBeTrue();
  });

  it('a folder taken from the picker waits for the save button', () => {
    store.system.set({ root: '/containers' } as SystemInfo);

    settings.pickFolder('root', '/home/vitorcds/containers');
    http.expectNone((r) => r.url === '/api/v1/system/root');
    expect(settings.dirty()).withContext('the button should be offering it').toBeTrue();

    settings.save();
    const save = http.expectOne((r) => r.method === 'PUT' && r.url === '/api/v1/system/root');
    expect(save.request.body).toEqual({ root: '/home/vitorcds/containers' });
    save.flush({ root: '/home/vitorcds/containers' } as SystemInfo);
    http.expectOne('/api/v1/instances').flush({ instances: [] });
  });

  it('one save writes the folder, the name and what is only local', () => {
    store.system.set({ root: '/containers' } as SystemInfo);
    settings.rootDraft.set('/mnt/jogos');
    settings.addName();
    settings.setName(1, 'outro');
    settings.setLanguage('en');
    settings.toggleMetric('disk');

    settings.save();

    const root = http.expectOne((r) => r.method === 'PUT' && r.url === '/api/v1/system/root');
    expect(root.request.body).toEqual({ root: '/mnt/jogos' });
    root.flush({ root: '/mnt/jogos' } as SystemInfo);

    const add = http.expectOne('/api/v1/dns/domains');
    expect(add.request.body).toEqual({ domain: 'outro' });
    add.flush({ domain: 'outro', hostname: 'outro.duckdns.org' });

    http.expectOne('/api/v1/instances').flush({ instances: [] });
    http.expectOne((r) => r.method === 'GET' && r.url === '/api/v1/dns').flush(dns);

    expect(TestBed.inject(I18n).pref()).toBe('en');
    expect(TestBed.inject(Prefs).metrics().disk).toBeFalse();
    expect(settings.dirty()).withContext('nothing left to save').toBeFalse();
  });

  it('nothing to save keeps the button quiet', () => {
    store.system.set({ root: '/containers' } as SystemInfo);
    settings.rootDraft.set('/containers');

    expect(settings.dirty()).toBeFalse();
    settings.save();
    http.expectNone(() => true);
  });

  it('shows the docker version, or that it did not answer', () => {
    store.system.set({ dockerVersion: '27.1.1' } as SystemInfo);
    expect(settings.dockerLabel()).toBe('versão 27.1.1');

    store.system.set({ dockerError: 'sem daemon' } as SystemInfo);
    expect(settings.dockerLabel()).toBe('não respondeu');
  });
});
