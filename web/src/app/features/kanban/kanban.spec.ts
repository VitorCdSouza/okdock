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
    dir: '/srv/games/smp',
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
    expect(pending!.instance.name).toBe('smp');
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

  it('archives on a drop into ARCHIVED, wherever the instance comes from', () => {
    store.instances.set([instance()]);
    store.dragging.set('smp');
    kanban.onDrop(dragEvent(), 'archived');

    expect(kanban.pendingAction()?.target).toBe('archived');

    kanban.confirmAction();

    const req = http.expectOne('/api/v1/instances/smp/archive');
    expect(req.request.method).toBe('POST');
    req.flush(null, { status: 202, statusText: 'Accepted' });

    http.expectOne('/api/v1/instances').flush({ instances: [], states: [] });
    http.expectOne('/api/v1/system').flush({});
  });

  it('refuses on ARCHIVED an instance that is already archived', () => {
    store.instances.set([instance({ state: 'archived', archived: true })]);
    store.dragging.set('smp');

    kanban.onDrop(dragEvent(), 'archived');

    expect(kanban.pendingAction()).toBeNull();
  });

  it('an external container takes neither update nor archive, but takes stop', () => {
    store.instances.set([instance({ external: true, project: 'media' })]);
    store.dragging.set('smp');

    kanban.onDrop(dragEvent(), 'updating');
    expect(kanban.pendingAction()).withContext('an external container has no image to update').toBeNull();

    store.dragging.set('smp');
    kanban.onDrop(dragEvent(), 'archived');
    expect(kanban.pendingAction()).withContext('an external container is not archived').toBeNull();

    store.dragging.set('smp');
    kanban.onDrop(dragEvent(), 'stopped');
    expect(kanban.pendingAction()?.target).toBe('stopped');
  });

  it('ignores the drop when the action would do nothing', () => {
    store.instances.set([instance({ state: 'stopped' })]);
    store.dragging.set('smp');

    kanban.onDrop(dragEvent(), 'stopped');

    expect(kanban.pendingAction()).toBeNull();
  });

  it('refuses an archived instance on UPDATING', () => {
    store.instances.set([instance({ state: 'archived', archived: true })]);
    store.dragging.set('smp');

    kanban.onDrop(dragEvent(), 'updating');

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

  it('the full column ends up wider than the empty one', () => {
    store.states.set(['running', 'stopped']);
    store.instances.set(
      Array.from({ length: 12 }, (_, i) => instance({ name: `smp-${i}` })),
    );

    const grow = new Map(kanban.columns().map((c) => [c.state, c.grow]));

    expect(grow.get('running')).toBe(3);
    expect(grow.get('stopped')).toBe(1);
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
