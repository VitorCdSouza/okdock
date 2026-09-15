import { TestBed } from '@angular/core/testing';
import { provideHttpClient } from '@angular/common/http';
import { provideHttpClientTesting, HttpTestingController } from '@angular/common/http/testing';

import { Kanban } from './kanban';
import { Store } from '../../core/state';
import { I18n } from '../../core/i18n/i18n';
import { Instance } from '../../core/models';

function instance(over: Partial<Instance> = {}): Instance {
  return {
    name: 'smp',
    templateId: 'minecraft-java',
    category: 'games',
    image: 'itzg/minecraft-server:java21',
    env: {},
    ports: [],
    mounts: [],
    memoryLimit: '4g',
    cpus: 2,
    restart: 'unless-stopped',
    stopGraceSeconds: 120,
    createdAt: '2026-08-21T00:00:00Z',
    updatedAt: '2026-08-21T00:00:00Z',
    dir: '/containers/smp',
    state: 'running',
    ...over,
  };
}

function dragEvent(): DragEvent {
  return new DragEvent('drop', { bubbles: true, cancelable: true, dataTransfer: new DataTransfer() });
}

describe('Kanban: dragging a card into a column', () => {
  let kanban: Kanban;
  let store: Store;
  let http: HttpTestingController;

  beforeEach(() => {
    TestBed.configureTestingModule({
      providers: [provideHttpClient(), provideHttpClientTesting()],
    });
    kanban = TestBed.createComponent(Kanban).componentInstance;
    store = TestBed.inject(Store);
    http = TestBed.inject(HttpTestingController);
    TestBed.inject(I18n).setPref('pt');
  });

  afterEach(() => http.verify());

  it('opens the confirmation when a running instance is dropped on UPDATING', () => {
    store.instances.set([instance()]);
    store.dragging.set('smp');

    kanban.onDrop(dragEvent(), 'updating');

    const pending = kanban.pendingAction();
    expect(pending).withContext('the confirmation did not open').not.toBeNull();
    expect(pending!.name).toBe('smp');
    expect(pending!.instances.map((i) => i.name)).toEqual(['smp']);
    expect(pending!.target).toBe('updating');
    expect(store.dragging()).withContext('the drag state should have been cleared').toBeNull();
  });

  it('opens the confirmation when dropped on STOPPED', () => {
    store.instances.set([instance()]);
    store.dragging.set('smp');

    kanban.onDrop(dragEvent(), 'stopped');

    expect(kanban.pendingAction()?.target).toBe('stopped');
  });

  it('opens the confirmation when a stopped instance is dropped on RUNNING', () => {
    store.instances.set([instance({ state: 'stopped' })]);
    store.dragging.set('smp');

    kanban.onDrop(dragEvent(), 'running');

    expect(kanban.pendingAction()?.target).toBe('running');
  });

  it('accepts an instance in error on RUNNING', () => {
    store.instances.set([instance({ state: 'error' })]);
    store.dragging.set('smp');

    kanban.onDrop(dragEvent(), 'running');

    expect(kanban.pendingAction()?.target).toBe('running');
  });

  it('refuses on RUNNING an instance that is already up', () => {
    store.instances.set([instance()]);
    store.dragging.set('smp');

    kanban.onDrop(dragEvent(), 'running');

    expect(kanban.pendingAction()).toBeNull();
  });

  it('confirming on RUNNING calls start', () => {
    store.instances.set([instance({ state: 'stopped' })]);
    store.dragging.set('smp');
    kanban.onDrop(dragEvent(), 'running');

    kanban.confirmAction();

    const req = http.expectOne('/api/v1/instances/smp/start');
    expect(req.request.method).toBe('POST');
    req.flush(null, { status: 202, statusText: 'Accepted' });

    expect(kanban.pendingAction()).toBeNull();
    http.expectOne('/api/v1/instances').flush({ instances: [], states: [] });
    http.expectOne('/api/v1/system').flush({});
  });

  it('an external container takes no update, but takes stop', () => {
    store.instances.set([instance({ external: true, project: 'media' })]);
    store.dragging.set('smp');

    kanban.onDrop(dragEvent(), 'updating');
    expect(kanban.pendingAction()).withContext('an external container has no image to update').toBeNull();

    store.dragging.set('smp');
    kanban.onDrop(dragEvent(), 'stopped');
    expect(kanban.pendingAction()?.target).toBe('stopped');
  });

  it('a stack dropped on a column takes every member the column accepts', () => {
    store.states.set(['running']);
    store.instances.set([
      instance({ name: 'nextcloud', external: true, project: 'nextcloud' }),
      instance({ name: 'nextcloud-db', external: true, project: 'nextcloud' }),
      instance({ name: 'jellyfin', external: true, project: 'media' }),
    ]);
    const group = kanban.columns()[0].items[0];
    if (group.kind !== 'group') throw new Error('the stack did not collapse into a tile');

    kanban.onGroupDragStart(new DragEvent('dragstart', { dataTransfer: new DataTransfer() }), group);
    expect(kanban.canDrop('stopped')).withContext('a stack that is up can be stopped').toBeTrue();
    expect(kanban.canDrop('updating')).withContext('an external container has no image to update').toBeFalse();

    kanban.onDrop(dragEvent(), 'stopped');

    const pending = kanban.pendingAction();
    expect(pending!.name).withContext('the dialog is about the stack').toBe('nextcloud');
    expect(pending!.instances.map((i) => i.name)).toEqual(['nextcloud', 'nextcloud-db']);

    kanban.confirmAction();

    http.expectOne('/api/v1/instances/nextcloud/stop').flush(null, { status: 202, statusText: 'Accepted' });
    http.expectOne('/api/v1/instances/nextcloud-db/stop').flush(null, { status: 202, statusText: 'Accepted' });
    expect(kanban.pendingAction()).toBeNull();
    http.expectOne('/api/v1/instances').flush({ instances: [], states: [] });
    http.expectOne('/api/v1/system').flush({});
  });

  it('a stack leaves behind the member the column refuses', () => {
    store.states.set(['running']);
    store.instances.set([
      instance({ name: 'nextcloud', external: true, project: 'nextcloud', editable: true }),
      instance({ name: 'nextcloud-db', external: true, project: 'nextcloud' }),
    ]);
    const group = kanban.columns()[0].items[0];
    if (group.kind !== 'group') throw new Error('the stack did not collapse into a tile');
    kanban.onGroupDragStart(new DragEvent('dragstart', { dataTransfer: new DataTransfer() }), group);

    kanban.onDrop(dragEvent(), 'updating');

    expect(kanban.pendingAction()!.instances.map((i) => i.name))
      .withContext('the member with no compose file the panel can read stays out')
      .toEqual(['nextcloud']);
  });

  it('ignores the drop when the action would do nothing', () => {
    store.instances.set([instance({ state: 'stopped' })]);
    store.dragging.set('smp');

    kanban.onDrop(dragEvent(), 'stopped');

    expect(kanban.pendingAction()).toBeNull();
  });

  it('the card button reloads the board when the call goes through', () => {
    const inst = instance({ external: true, project: 'media' });
    store.instances.set([inst]);

    kanban.onAction({ instance: inst, verb: 'stop' });

    http.expectOne('/api/v1/instances/smp/stop').flush(null, { status: 202, statusText: 'Accepted' });
    http.expectOne('/api/v1/instances').flush({ instances: [], states: [] });
    http.expectOne('/api/v1/system').flush({});
  });

  it('the card button warns when the call is refused', () => {
    const inst = instance({ external: true, project: 'media' });
    store.instances.set([inst]);

    kanban.onAction({ instance: inst, verb: 'stop' });

    http.expectOne('/api/v1/instances/smp/stop').flush(
      { error: 'external_instance', message: 'externo', params: { name: 'smp' } },
      { status: 409, statusText: 'Conflict' },
    );

    expect(store.toast()).withContext('a swallowed refusal makes the button look dead').toContain('container externo');
    expect(store.toastBad()).toBeTrue();
  });

  it('containers of the same stack collapse into one tile, and single ones stay as cards', () => {
    store.states.set(['running']);
    store.instances.set([
      instance({ name: 'smp' }),
      instance({ name: 'nextcloud', external: true, project: 'nextcloud' }),
      instance({ name: 'jellyfin', external: true, project: 'media' }),
      instance({ name: 'nextcloud-db', external: true, project: 'nextcloud' }),
    ]);

    const items = kanban.columns()[0].items;

    expect(items.map((i) => i.key))
      .withContext('the tiles come first, the loose cards after')
      .toEqual(['running:nextcloud', 'smp', 'jellyfin']);
    const group = items[0];
    expect(group.kind).toBe('group');
    if (group.kind !== 'group') return;
    expect(group.members.map((m) => m.name)).toEqual(['nextcloud', 'nextcloud-db']);
    expect(group.icons.length).withContext('the tile shows at most four icons').toBe(2);
    expect(group.summary).withContext('the closed tile names what is inside').toBe('nextcloud, nextcloud-db');
    expect(group.open).withContext('a group is born closed').toBeFalse();
  });

  it('groups open side by side, and the tile closes each one again', () => {
    store.states.set(['running']);
    store.instances.set([
      instance({ name: 'nextcloud', external: true, project: 'nextcloud' }),
      instance({ name: 'nextcloud-db', external: true, project: 'nextcloud' }),
      instance({ name: 'jellyfin', external: true, project: 'media' }),
      instance({ name: 'jellyfin-db', external: true, project: 'media' }),
    ]);

    kanban.toggleGroup('running:nextcloud');
    expect(kanban.columns()[0].items.map((i) => i.kind === 'group' && i.open)).toEqual([true, false]);

    kanban.toggleGroup('running:media');
    expect(kanban.columns()[0].items.map((i) => i.kind === 'group' && i.open))
      .withContext('opening one group cannot close the other')
      .toEqual([true, true]);

    kanban.toggleGroup('running:media');
    expect(kanban.columns()[0].items.map((i) => i.kind === 'group' && i.open)).toEqual([true, false]);

    kanban.closeGroups();
    expect(kanban.columns()[0].items.map((i) => i.kind === 'group' && i.open)).toEqual([false, false]);
  });

  it('the same stack split between two columns opens one column at a time', () => {
    store.states.set(['running', 'stopped']);
    store.instances.set([
      instance({ name: 'nextcloud', external: true, project: 'nextcloud' }),
      instance({ name: 'nextcloud-db', external: true, project: 'nextcloud' }),
      instance({ name: 'nextcloud-cron', external: true, project: 'nextcloud', state: 'stopped' }),
      instance({ name: 'nextcloud-redis', external: true, project: 'nextcloud', state: 'stopped' }),
    ]);

    kanban.toggleGroup('running:nextcloud');

    const open = new Map(
      kanban.columns().map((c) => [c.state, c.items.map((i) => i.kind === 'group' && i.open)]),
    );
    expect(open.get('running')).toEqual([true]);
    expect(open.get('stopped')).withContext('the stopped half stays closed').toEqual([false]);
  });

  it('stopped and running keep the width of two cards, full or empty', () => {
    store.states.set(['stopped', 'running', 'updating', 'error']);
    store.instances.set(
      Array.from({ length: 12 }, (_, i) =>
        instance({ name: `media-${i}`, external: true, project: 'media' }),
      ),
    );

    const grow = new Map(kanban.columns().map((c) => [c.state, c.grow]));
    expect(grow.get('running')).toBe(2);
    expect(grow.get('stopped')).withContext('an empty stopped column is just as wide').toBe(2);
    expect(grow.get('updating')).toBe(1);
    expect(grow.get('error')).toBe(1);

    kanban.toggleGroup('running:media');
    store.categoryFilter.set('network');

    const after = new Map(kanban.columns().map((c) => [c.state, c.grow]));
    expect(after.get('running')).withContext('neither an open group nor a filter resizes it').toBe(2);
    expect(after.get('stopped')).toBe(2);
  });

  it('confirming fires the right call and closes the dialog', () => {
    store.instances.set([instance()]);
    store.dragging.set('smp');
    kanban.onDrop(dragEvent(), 'updating');

    kanban.confirmAction();

    const req = http.expectOne('/api/v1/instances/smp/update-image');
    expect(req.request.method).toBe('POST');
    req.flush(null, { status: 202, statusText: 'Accepted' });

    expect(kanban.pendingAction()).toBeNull();
    http.expectOne('/api/v1/instances').flush({ instances: [], states: [] });
    http.expectOne('/api/v1/system').flush({});
  });
});
