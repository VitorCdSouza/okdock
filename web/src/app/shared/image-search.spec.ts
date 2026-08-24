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

  // NgModel applies a disabled binding in a microtask, hence the ticks
  it('shows two labelled boxes, and the version only wakes up with an image', fakeAsync(() => {
    const fixture = TestBed.createComponent(ImageSearch);
    fixture.componentRef.setInput('image', '');
    fixture.componentRef.setInput('label', 'Imagem');
    fixture.componentRef.setInput('required', true);
    fixture.detectChanges();
    tick();

    const labels: HTMLElement[] = Array.from(fixture.nativeElement.querySelectorAll('label'));
    expect(labels.map((l) => l.textContent!.trim().replace(/\s+/g, ' '))).toEqual([
      'Imagem *',
      'versão',
    ]);

    const inputs: HTMLInputElement[] = Array.from(fixture.nativeElement.querySelectorAll('input'));
    expect(inputs.length).toBe(2);
    expect(inputs[1].disabled).toBeTrue();
    expect(fixture.nativeElement.querySelectorAll('.caret').length).toBe(2);

    fixture.componentRef.setInput('image', 'jellyfin/jellyfin');
    fixture.detectChanges();
    tick();
    expect(inputs[1].disabled).toBeFalse();
  }));

  it('waits for the typing to settle before asking the daemon', fakeAsync(() => {
    const c = field();
    c.typeRepo('jel');
    c.typeRepo('jelly');
    c.typeRepo('jellyfin');
    tick(300);

    const req = http.expectOne('/api/v1/images?q=jellyfin');
    req.flush({ images: [{ name: 'jellyfin/jellyfin', description: 'media', stars: 1200 }] });

    expect(c.hits().length).toBe(1);
  }));

  it('asks nothing for a term too short to mean anything', fakeAsync(() => {
    field().typeRepo('j');
    tick(300);

    http.expectNone(() => true);
  }));

  it('a whole reference pasted in the image box splits itself', fakeAsync(() => {
    const c = field();
    c.typeRepo('jellyfin/jellyfin:10.9.11');
    tick(300);

    expect(c.repo()).toBe('jellyfin/jellyfin');
    expect(c.tag()).toBe('10.9.11');
    // the search is by repository, the tag is not part of the term
    http.expectOne('/api/v1/images?q=jellyfin%2Fjellyfin').flush({ images: [] });
  }));

  it('the version box has nothing to offer before the image is filled', fakeAsync(() => {
    const c = field();
    c.openTags();
    tick(300);

    http.expectNone(() => true);
    expect(c.open()).toBeNull();
  }));

  it('the caret opens the repository list without waiting for a keystroke', fakeAsync(() => {
    const c = field('jellyfin');
    c.openRepos();
    tick(300);

    http.expectOne('/api/v1/images?q=jellyfin').flush({
      images: [{ name: 'jellyfin/jellyfin', description: '', stars: 10 }],
    });
    expect(c.open()).toBe('repo');
    expect(c.hits().length).toBe(1);
  }));

  it('picking a repository moves on to the version, filtered by what is typed', fakeAsync(() => {
    const c = field();
    c.typeRepo('jellyfin');
    tick(300);
    http.expectOne('/api/v1/images?q=jellyfin').flush({
      images: [{ name: 'linuxserver/jellyfin', description: '', stars: 800 }],
    });

    c.pick({ name: 'linuxserver/jellyfin', description: '', stars: 800 });
    expect(c.image()).toBe('linuxserver/jellyfin');
    expect(c.open()).toBe('tag');

    tick(300);
    http.expectOne('/api/v1/images/tags?image=linuxserver%2Fjellyfin').flush({
      tags: ['latest', '10.9.11', '10.8.13'],
    });
    expect(c.tags()).toEqual(['latest', '10.9.11', '10.8.13']);

    // typing in the version box filters what is already loaded, no new request
    c.typeTag('10.9');
    tick(300);
    expect(c.tags()).toEqual(['10.9.11']);
    expect(c.image()).toBe('linuxserver/jellyfin:10.9');

    c.pickTag('10.9.11');
    expect(c.image()).toBe('linuxserver/jellyfin:10.9.11');
    expect(c.open()).toBeNull();
  }));

  it('does not ask the Hub for tags of an image the Hub does not host', fakeAsync(() => {
    const c = field('lscr.io/linuxserver/jellyfin');
    c.openTags();
    tick(300);

    http.expectNone(() => true);
    expect(c.notHub()).toBeTrue();
  }));

  it('closing stays closed even when the reply lands after it', fakeAsync(() => {
    const c = field();
    c.typeRepo('jellyfin');
    tick(300);
    const req = http.expectOne('/api/v1/images?q=jellyfin');

    c.close();
    req.flush({ images: [{ name: 'jellyfin/jellyfin', description: '', stars: 10 }] });

    expect(c.open()).toBeNull();
  }));

  it('a registry that did not answer says so and keeps searching', fakeAsync(() => {
    const c = field();
    c.typeRepo('jellyfin');
    tick(300);
    http.expectOne('/api/v1/images?q=jellyfin').flush('boom', { status: 409, statusText: 'Conflict' });

    expect(c.failed()).toBeTrue();
    expect(c.busy()).toBeFalse();
    expect(c.image()).toBe('jellyfin');

    // the failure cannot kill the stream, or the field never searches again
    c.typeRepo('nextcloud');
    tick(300);
    http.expectOne('/api/v1/images?q=nextcloud').flush({
      images: [{ name: 'nextcloud', description: '', stars: 4000 }],
    });

    expect(c.failed()).toBeFalse();
    expect(c.hits().length).toBe(1);
  }));
});
