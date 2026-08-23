import { TestBed } from '@angular/core/testing';
import { provideHttpClient } from '@angular/common/http';
import { provideHttpClientTesting, HttpTestingController } from '@angular/common/http/testing';

import { Kanban } from './kanban';
import { Store } from '../../core/state';
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

describe('Kanban — arrastar card para uma coluna', () => {
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
  });

  afterEach(() => http.verify());

  it('abre a confirmação ao soltar uma instância de pé em ATUALIZANDO', () => {
    store.instances.set([instance()]);
    store.dragging.set('smp');

    kanban.onDrop(dragEvent(), 'updating');

    const pending = kanban.pendingAction();
    expect(pending).withContext('confirmação não abriu').not.toBeNull();
    expect(pending!.instance.name).toBe('smp');
    expect(pending!.target).toBe('updating');
    expect(store.dragging()).withContext('estado de arrasto devia ter sido limpo').toBeNull();
  });

  it('abre a confirmação ao soltar em PARADO', () => {
    store.instances.set([instance()]);
    store.dragging.set('smp');

    kanban.onDrop(dragEvent(), 'stopped');

    expect(kanban.pendingAction()?.target).toBe('stopped');
  });

  it('abre a confirmação ao soltar uma instância parada em RODANDO', () => {
    store.instances.set([instance({ state: 'stopped' })]);
    store.dragging.set('smp');

    kanban.onDrop(dragEvent(), 'running');

    expect(kanban.pendingAction()?.target).toBe('running');
  });

  it('aceita em RODANDO uma instância em erro', () => {
    store.instances.set([instance({ state: 'error' })]);
    store.dragging.set('smp');

    kanban.onDrop(dragEvent(), 'running');

    expect(kanban.pendingAction()?.target).toBe('running');
  });

  it('não aceita em RODANDO uma instância que já está de pé', () => {
    store.instances.set([instance()]);
    store.dragging.set('smp');

    kanban.onDrop(dragEvent(), 'running');

    expect(kanban.pendingAction()).toBeNull();
  });

  it('confirmar em RODANDO chama start', () => {
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

  it('arquiva ao soltar em ARQUIVADA, venha a instância de onde vier', () => {
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

  it('não aceita em ARQUIVADA uma instância já arquivada', () => {
    store.instances.set([instance({ state: 'archived', archived: true })]);
    store.dragging.set('smp');

    kanban.onDrop(dragEvent(), 'archived');

    expect(kanban.pendingAction()).toBeNull();
  });

  it('container externo não aceita atualizar nem arquivar, mas aceita parar', () => {
    store.instances.set([instance({ external: true, project: 'media' })]);
    store.dragging.set('smp');

    kanban.onDrop(dragEvent(), 'updating');
    expect(kanban.pendingAction()).withContext('externo não tem imagem para atualizar').toBeNull();

    store.dragging.set('smp');
    kanban.onDrop(dragEvent(), 'archived');
    expect(kanban.pendingAction()).withContext('externo não se arquiva').toBeNull();

    store.dragging.set('smp');
    kanban.onDrop(dragEvent(), 'stopped');
    expect(kanban.pendingAction()?.target).toBe('stopped');
  });

  it('ignora o drop quando a ação não faria nada', () => {
    store.instances.set([instance({ state: 'stopped' })]);
    store.dragging.set('smp');

    kanban.onDrop(dragEvent(), 'stopped');

    expect(kanban.pendingAction()).toBeNull();
  });

  it('não aceita instância arquivada em ATUALIZANDO', () => {
    store.instances.set([instance({ state: 'archived', archived: true })]);
    store.dragging.set('smp');

    kanban.onDrop(dragEvent(), 'updating');

    expect(kanban.pendingAction()).toBeNull();
  });

  it('confirmar dispara a chamada certa e fecha o diálogo', () => {
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
