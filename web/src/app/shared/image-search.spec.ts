import { TestBed, fakeAsync, tick } from '@angular/core/testing';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { provideHttpClient } from '@angular/common/http';

import { ImageSearch } from './image-search';
import { I18n } from '../core/i18n/i18n';

describe('ImageSearch', () => {
  let http: HttpTestingController;

  function field(initial = ''): ImageSearch {
    const fixture = TestBed.createComponent(ImageSearch);
    fixture.componentRef.setInput('image', initial);
    fixture.detectChanges();
    return fixture.componentInstance;
  }

  beforeEach(() => {
    localStorage.removeItem('okdock.locale');
    TestBed.configureTestingModule({
      providers: [provideHttpClient(), provideHttpClientTesting()],
    });
    TestBed.inject(I18n).setPref('pt');
    http = TestBed.inject(HttpTestingController);
  });

  afterEach(() => {
    http.verify();
    localStorage.removeItem('okdock.locale');
  });

  it('waits for the typing to settle before asking the daemon', fakeAsync(() => {
    const c = field();
    c.type('jel');
    c.type('jelly');
    c.type('jellyfin');
    tick(300);

    const req = http.expectOne('/api/v1/images?q=jellyfin');
    req.flush({ images: [{ name: 'jellyfin/jellyfin', description: 'media', stars: 1200 }] });

    expect(c.hits().length).toBe(1);
  }));

  it('searches the repository, not the tag already typed', fakeAsync(() => {
    field().type('jellyfin/jellyfin:10.9');
    tick(300);

    http.expectOne('/api/v1/images?q=jellyfin%2Fjellyfin').flush({ images: [] });
  }));

  it('asks nothing for a term too short to mean anything', fakeAsync(() => {
    field().type('j');
    tick(300);

    http.expectNone(() => true);
  }));

  it('picking a repository keeps the tag the user had typed', fakeAsync(() => {
    const c = field();
    c.type('jellyfin:10.9');
    tick(300);
    http.expectOne('/api/v1/images?q=jellyfin').flush({
      images: [{ name: 'linuxserver/jellyfin', description: '', stars: 800 }],
    });

    c.pick({ name: 'linuxserver/jellyfin', description: '', stars: 800 });

    expect(c.image()).toBe('linuxserver/jellyfin:10.9');
    expect(c.hits().length).toBe(0);
  }));

  it('a registry that did not answer says so and keeps searching', fakeAsync(() => {
    const c = field();
    c.type('jellyfin');
    tick(300);
    http.expectOne('/api/v1/images?q=jellyfin').flush('boom', { status: 409, statusText: 'Conflict' });

    expect(c.failed()).toBeTrue();
    expect(c.busy()).toBeFalse();
    expect(c.image()).toBe('jellyfin');

    // the failure cannot kill the stream, or the field never searches again
    c.type('nextcloud');
    tick(300);
    http.expectOne('/api/v1/images?q=nextcloud').flush({
      images: [{ name: 'nextcloud', description: '', stars: 4000 }],
    });

    expect(c.failed()).toBeFalse();
    expect(c.hits().length).toBe(1);
  }));
});
