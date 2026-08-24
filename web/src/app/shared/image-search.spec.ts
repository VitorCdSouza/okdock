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

  it('a colon turns the search into the tag list of that repository', fakeAsync(() => {
    const c = field();
    c.type('jellyfin/jellyfin:10');
    tick(300);

    http.expectOne('/api/v1/images/tags?image=jellyfin%2Fjellyfin').flush({
      tags: ['10.9.11', '10.8.13', 'latest'],
    });

    // what was typed after the colon filters the list, with no second request
    expect(c.tags()).toEqual(['10.9.11', '10.8.13']);
  }));

  it('does not ask the Hub for tags of an image the Hub does not host', fakeAsync(() => {
    const c = field();
    c.type('lscr.io/linuxserver/jellyfin:');
    tick(300);

    http.expectNone(() => true);
    expect(c.notHub()).toBeTrue();
  }));

  it('asks nothing for a term too short to mean anything', fakeAsync(() => {
    field().type('j');
    tick(300);

    http.expectNone(() => true);
  }));

  it('picking a repository leaves a valid reference and offers its tags', fakeAsync(() => {
    const c = field();
    c.type('jellyfin');
    tick(300);
    http.expectOne('/api/v1/images?q=jellyfin').flush({
      images: [{ name: 'linuxserver/jellyfin', description: '', stars: 800 }],
    });

    c.pick({ name: 'linuxserver/jellyfin', description: '', stars: 800 });
    expect(c.image()).toBe('linuxserver/jellyfin');
    expect(c.hits().length).toBe(0);

    tick(300);
    http.expectOne('/api/v1/images/tags?image=linuxserver%2Fjellyfin').flush({
      tags: ['latest', '10.9.11'],
    });
    expect(c.tags()).toEqual(['latest', '10.9.11']);

    c.pickTag('10.9.11');
    expect(c.image()).toBe('linuxserver/jellyfin:10.9.11');
    expect(c.tags().length).toBe(0);
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
