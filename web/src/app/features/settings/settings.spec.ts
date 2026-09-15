import { TestBed } from '@angular/core/testing';
import { provideHttpClient } from '@angular/common/http';
import { provideHttpClientTesting, HttpTestingController } from '@angular/common/http/testing';

import { Settings } from './settings';
import { Store } from '../../core/state';
import { I18n } from '../../core/i18n/i18n';
import { Prefs } from '../../core/prefs';
import { SystemInfo } from '../../core/models';

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
  });

  afterEach(() => {
    localStorage.removeItem('okdock.locale');
    localStorage.removeItem('okdock.metrics');
    localStorage.removeItem('okdock.showSelf');
  });

  it('showing the panel itself waits for the save button too', () => {
    const prefs = TestBed.inject(Prefs);
    prefs.showSelf.set(false);

    settings.showSelfDraft.set(true);
    expect(settings.dirty()).toBeTrue();
    expect(prefs.showSelf()).withContext('not before saving').toBeFalse();

    settings.save();
    expect(prefs.showSelf()).toBeTrue();
    expect(settings.dirty()).toBeFalse();
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

  it('one save writes the folder and what is only local', () => {
    store.system.set({ root: '/containers' } as SystemInfo);
    settings.rootDraft.set('/mnt/jogos');
    settings.setLanguage('en');
    settings.toggleMetric('disk');

    settings.save();

    const root = http.expectOne((r) => r.method === 'PUT' && r.url === '/api/v1/system/root');
    expect(root.request.body).toEqual({ root: '/mnt/jogos' });
    root.flush({ root: '/mnt/jogos' } as SystemInfo);

    http.expectOne('/api/v1/instances').flush({ instances: [] });

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
