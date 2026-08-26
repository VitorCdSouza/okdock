import { ComponentRef } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { provideHttpClient } from '@angular/common/http';
import { provideHttpClientTesting, HttpTestingController } from '@angular/common/http/testing';

import { DirPicker, GhostDir, TreeNode } from './dir-picker';

const ROOT = '/home/vitorcds/servidor';

describe('DirPicker: the folder tree', () => {
  let fixture: ComponentFixture<DirPicker>;
  let screen: DirPicker;
  let ref: ComponentRef<DirPicker>;
  let http: HttpTestingController;

  function url(path: string): string {
    return `/api/v1/fs?path=${encodeURIComponent(path)}`;
  }

  function node(path: string): TreeNode {
    return screen.nodes().find((n) => n.path === path)!;
  }

  function start(ghosts: GhostDir[] = []): void {
    ref.setInput('start', ROOT);
    ref.setInput('ghosts', ghosts);
    fixture.detectChanges();
    http.expectOne(url(ROOT)).flush({
      path: ROOT,
      roots: [ROOT, '/containers'],
      entries: [
        { name: 'media', path: `${ROOT}/media` },
        { name: 'nextcloud', path: `${ROOT}/nextcloud` },
      ],
    });
  }

  beforeEach(() => {
    localStorage.removeItem('okdock.locale');
    TestBed.configureTestingModule({
      providers: [provideHttpClient(), provideHttpClientTesting()],
    });
    fixture = TestBed.createComponent(DirPicker);
    screen = fixture.componentInstance;
    ref = fixture.componentRef;
    http = TestBed.inject(HttpTestingController);
  });

  afterEach(() => http.verify());

  it('opens the root it was given and leaves the other one closed', () => {
    start();

    expect(screen.nodes().map((n) => n.name)).toEqual([ROOT, 'media', 'nextcloud', '/containers']);
    expect(node(ROOT).depth).toBe(0);
    expect(node(`${ROOT}/media`).depth).toBe(1);
    expect(screen.selected()).toBe(ROOT);
  });

  it('asks for the children of a folder the first time it is opened', () => {
    start();

    screen.toggle(node(`${ROOT}/media`));
    http.expectOne(url(`${ROOT}/media`)).flush({
      path: `${ROOT}/media`,
      parent: ROOT,
      roots: [ROOT],
      entries: [{ name: 'filmes', path: `${ROOT}/media/filmes` }],
    });

    expect(node(`${ROOT}/media/filmes`).depth).toBe(2);

    screen.toggle(node(`${ROOT}/media`));
    screen.toggle(node(`${ROOT}/media`));
    // the answer is kept, opening it again asks nothing
    http.expectNone(url(`${ROOT}/media`));
  });

  it('hangs the folder that does not exist yet on the tree, open', () => {
    start([{ path: `${ROOT}/smp` }, { path: `${ROOT}/smp/data` }]);

    expect(node(`${ROOT}/smp`).ghost).toBeTrue();
    // nothing to list, the children of a ghost are ghosts, and they show without a click
    http.expectNone(url(`${ROOT}/smp`));
    expect(node(`${ROOT}/smp/data`).depth).toBe(2);
    expect(node(`${ROOT}/smp/data`).leaf).toBeTrue();

    screen.toggle(node(`${ROOT}/smp`));
    expect(screen.nodes().some((n) => n.path === `${ROOT}/smp/data`)).toBeFalse();
  });

  it('creates a folder inside the one that is selected and keeps it selected', () => {
    start();
    screen.select(`${ROOT}/media`);
    screen.startNaming();
    screen.newName.set('mundos');

    screen.make();

    const post = http.expectOne('/api/v1/fs');
    expect(post.request.body).toEqual({ path: `${ROOT}/media/mundos` });
    post.flush({ path: `${ROOT}/media/mundos` }, { status: 201, statusText: 'Created' });
    http.expectOne(url(`${ROOT}/media`)).flush({
      path: `${ROOT}/media`,
      parent: ROOT,
      roots: [ROOT],
      entries: [{ name: 'mundos', path: `${ROOT}/media/mundos` }],
    });

    expect(screen.selected()).toBe(`${ROOT}/media/mundos`);
    expect(screen.naming()).toBeFalse();
  });

  it('hands back the folder that is selected', () => {
    start();
    const picked: string[] = [];
    screen.picked.subscribe((path) => picked.push(path));

    screen.select(`${ROOT}/media`);
    screen.pick();

    expect(picked).toEqual([`${ROOT}/media`]);
  });
});
