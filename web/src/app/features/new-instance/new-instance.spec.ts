import { TestBed } from '@angular/core/testing';
import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';

import { NewInstance } from './new-instance';
import { Store } from '../../core/state';
import { Instance, SystemInfo, Template } from '../../core/models';

function template(over: Partial<Template> = {}): Template {
  return {
    id: 'minecraft-java',
    name: 'Minecraft (Java)',
    category: 'games',
    short: 'MC',
    image: 'itzg/minecraft-server:java21',
    ports: [{ container: 25565, protocol: 'tcp', label: 'game' }],
    volumes: [{ container: '/data' }],
    defaultMemory: '4g',
    minMemory: '2g',
    defaultCpus: 2,
    stopGraceSeconds: 120,
    fields: [],
    builtin: true,
    ...over,
  };
}

function instance(name: string, host: number): Instance {
  return {
    name,
    templateId: 'minecraft-java',
    category: 'games',
    image: 'itzg/minecraft-server:java21',
    env: {},
    ports: [{ host, container: 25565, protocol: 'tcp', label: 'game' }],
    mounts: [],
    memoryLimit: '4g',
    cpus: 2,
    restart: 'unless-stopped',
    stopGraceSeconds: 120,
    createdAt: '2026-08-26T12:00:00Z',
    updatedAt: '2026-08-26T12:00:00Z',
    dir: `/containers/${name}`,
    state: 'running',
  };
}

describe('NewInstance: the host port the form starts with', () => {
  let screen: NewInstance;
  let store: Store;

  beforeEach(() => {
    localStorage.removeItem('okdock.locale');
    TestBed.configureTestingModule({
      providers: [provideHttpClient(), provideHttpClientTesting()],
    });
    screen = TestBed.createComponent(NewInstance).componentInstance;
    store = TestBed.inject(Store);
  });

  it('offers the port the template asked for', () => {
    screen.pick(template());

    expect(screen.ports().map((p) => p.host)).toEqual([25565]);
  });

  it('offers it even when another instance holds it', () => {
    store.instances.set([instance('smp', 25565)]);

    screen.pick(template());

    expect(screen.ports()[0].host).toBe(25565);
  });

  it('offers every port the template declared', () => {
    screen.pick(
      template({
        ports: [
          { container: 25565, protocol: 'tcp', label: 'game' },
          { container: 25565, protocol: 'udp', label: 'voice' },
          { container: 25575, protocol: 'tcp', label: 'rcon' },
        ],
      }),
    );

    expect(screen.ports().map((p) => p.host)).toEqual([25565, 25565, 25575]);
  });

  it('offers a folder named after the path in the container', () => {
    screen.pick(template({ volumes: [{ container: '/data' }, { container: '/config' }] }));

    expect(screen.volumes().map((v) => v.host)).toEqual(['./data', './config']);
  });

  it('does not put two volumes in the same folder', () => {
    screen.pick(template({ volumes: [{ container: '/data' }, { container: '/opt/data' }] }));

    expect(screen.volumes().map((v) => v.host)).toEqual(['./data', './opt-data']);
  });

  it('sends the folder that was typed', () => {
    screen.pick(template({ volumes: [{ container: '/data' }] }));
    screen.setVolume('/data', '/containers/mundos');

    expect(screen.volumes()[0].host).toBe('/containers/mundos');
  });

  it('turns a folder under the instance into a relative one', () => {
    store.system.set({ root: '/home/vitorcds/servidor' } as SystemInfo);
    screen.pick(template({ volumes: [{ container: '/data' }] }));
    screen.name.set('smp');

    screen.dirPicked('/data', '/home/vitorcds/servidor/smp/data');

    expect(screen.volumes()[0].host).toBe('./data');
  });

  it('keeps a folder outside the instance as it is', () => {
    store.system.set({ root: '/home/vitorcds/servidor' } as SystemInfo);
    screen.pick(template({ volumes: [{ container: '/data' }] }));
    screen.name.set('smp');

    screen.dirPicked('/data', '/home/vitorcds/servidor/media/filmes');

    expect(screen.volumes()[0].host).toBe('/home/vitorcds/servidor/media/filmes');
  });

  it('offers the instance folder and its volumes as folders to be created', () => {
    store.system.set({ root: '/home/vitorcds/servidor' } as SystemInfo);
    screen.pick(template({ volumes: [{ container: '/data' }, { container: '/config' }] }));
    screen.name.set('smp');

    expect(screen.ghostDirs().map((g) => g.path)).toEqual([
      '/home/vitorcds/servidor/smp',
      '/home/vitorcds/servidor/smp/data',
      '/home/vitorcds/servidor/smp/config',
    ]);
  });

  it('keeps what was typed by hand', () => {
    screen.pick(template());
    screen.setPort(25565, 'tcp', '30000');

    expect(screen.ports()[0].host).toBe(30000);
  });
});

describe('NewInstance: what the form adds on top of the template', () => {
  let screen: NewInstance;
  let http: HttpTestingController;

  beforeEach(() => {
    localStorage.removeItem('okdock.locale');
    TestBed.configureTestingModule({
      providers: [provideHttpClient(), provideHttpClientTesting()],
    });
    screen = TestBed.createComponent(NewInstance).componentInstance;
    http = TestBed.inject(HttpTestingController);
  });

  afterEach(() => http.verify());

  it('opens a new port on the host under the same number', () => {
    screen.pick(template());
    screen.addPort();
    screen.setExtraPortContainer(screen.extraPorts()[0].id, '8080');

    expect(screen.extraPorts()[0].host).toBe('8080');
  });

  it('stops following the container port once the host one is typed', () => {
    screen.pick(template());
    screen.addPort();
    const id = screen.extraPorts()[0].id;
    screen.setExtraPortHost(id, '30000');
    screen.setExtraPortContainer(id, '8080');

    expect(screen.extraPorts()[0].host).toBe('30000');
  });

  it('names the folder of a volume that was added by hand', () => {
    screen.pick(template());
    screen.addVolume();
    screen.setExtraVolumeContainer(screen.extraVolumes()[0].id, '/config');

    expect(screen.extraVolumes()[0].host).toBe('./config');
  });

  it('sends the port, the volume and the variable that were added', () => {
    screen.pick(template());
    screen.name.set('smp');

    screen.addPort();
    screen.setExtraPortContainer(screen.extraPorts()[0].id, '25575');
    screen.setExtraPortProtocol(screen.extraPorts()[0].id, 'udp');

    screen.addVolume();
    screen.setExtraVolumeContainer(screen.extraVolumes()[0].id, '/config');

    screen.addField();
    screen.setExtraFieldKey(screen.extraFields()[0].id, 'TZ');
    screen.setExtraFieldValue(screen.extraFields()[0].id, 'America/Sao_Paulo');

    screen.step.set(2);
    screen.next();

    const post = http.expectOne('/api/v1/instances');
    expect(post.request.body.ports).toEqual([
      { host: 25565, container: 25565, protocol: 'tcp', label: 'game' },
      { host: 25575, container: 25575, protocol: 'udp', label: '' },
    ]);
    expect(post.request.body.mounts).toEqual([
      { host: './data', container: '/data' },
      { host: './config', container: '/config' },
    ]);
    expect(post.request.body.extraEnv).toEqual({ TZ: 'America/Sao_Paulo' });
    post.flush(instance('smp', 25565));
  });

  it('leaves out a row that was added and never filled', () => {
    screen.pick(template());
    screen.name.set('smp');
    screen.addPort();
    screen.addVolume();
    screen.addField();

    screen.step.set(2);
    screen.next();

    const post = http.expectOne('/api/v1/instances');
    expect(post.request.body.ports.length).toBe(1);
    expect(post.request.body.mounts.length).toBe(1);
    expect(post.request.body.extraEnv).toEqual({});
    post.flush(instance('smp', 25565));
  });
});
