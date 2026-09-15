import { ComponentFixture, TestBed } from '@angular/core/testing';
import { provideHttpClient } from '@angular/common/http';
import { provideHttpClientTesting } from '@angular/common/http/testing';

import { InstanceForm, InstanceFormSeed } from './instance-form';
import { Store } from '../core/state';
import { Instance, SystemInfo, Template } from '../core/models';

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

function seedOf(inst: Instance): InstanceFormSeed {
  return {
    name: inst.name,
    image: inst.image,
    memoryLimit: inst.memoryLimit,
    cpus: inst.cpus,
    env: { ...inst.env },
    ports: inst.ports,
    mounts: inst.mounts,
  };
}

describe('InstanceForm: what the template puts on the screen', () => {
  let fixture: ComponentFixture<InstanceForm>;
  let form: InstanceForm;
  let store: Store;

  function open(p: Template, seed: InstanceFormSeed | null = null): void {
    fixture.componentRef.setInput('template', p);
    fixture.componentRef.setInput('seed', seed);
    fixture.detectChanges();
  }

  beforeEach(() => {
    localStorage.removeItem('okdock.locale');
    TestBed.configureTestingModule({
      providers: [provideHttpClient(), provideHttpClientTesting()],
    });
    fixture = TestBed.createComponent(InstanceForm);
    form = fixture.componentInstance;
    store = TestBed.inject(Store);
  });

  it('offers the port the template asked for', () => {
    open(template());

    expect(form.value().ports.map((p) => p.host)).toEqual([25565]);
  });

  it('offers it even when another instance holds it', () => {
    store.instances.set([instance('smp', 25565)]);

    open(template());

    expect(form.value().ports[0].host).toBe(25565);
  });

  it('offers every port the template declared', () => {
    open(
      template({
        ports: [
          { container: 25565, protocol: 'tcp', label: 'game' },
          { container: 25565, protocol: 'udp', label: 'voice' },
          { container: 25575, protocol: 'tcp', label: 'rcon' },
        ],
      }),
    );

    expect(form.value().ports.map((p) => p.host)).toEqual([25565, 25565, 25575]);
  });

  it('offers a folder named after the path in the container', () => {
    open(template({ volumes: [{ container: '/data' }, { container: '/config' }] }));

    expect(form.value().mounts.map((m) => m.host)).toEqual(['./data', './config']);
  });

  it('does not put two volumes in the same folder', () => {
    open(template({ volumes: [{ container: '/data' }, { container: '/opt/data' }] }));

    expect(form.value().mounts.map((m) => m.host)).toEqual(['./data', './opt-data']);
  });

  it('sends the folder that was typed', () => {
    open(template({ volumes: [{ container: '/data' }] }));
    form.setVolumeHost(form.volumes()[0].id, '/containers/mundos');

    expect(form.value().mounts[0].host).toBe('/containers/mundos');
  });

  it('turns a folder under the instance into a relative one', () => {
    store.system.set({ root: '/home/vitorcds/servidor' } as SystemInfo);
    open(template({ volumes: [{ container: '/data' }] }));
    form.name.set('smp');

    form.volumePicked(form.volumes()[0].id, '/home/vitorcds/servidor/smp/data');

    expect(form.value().mounts[0].host).toBe('./data');
  });

  it('keeps a folder outside the instance as it is', () => {
    store.system.set({ root: '/home/vitorcds/servidor' } as SystemInfo);
    open(template({ volumes: [{ container: '/data' }] }));
    form.name.set('smp');

    form.volumePicked(form.volumes()[0].id, '/home/vitorcds/servidor/media/filmes');

    expect(form.value().mounts[0].host).toBe('/home/vitorcds/servidor/media/filmes');
  });

  it('offers the instance folder and its volumes as folders to be created', () => {
    store.system.set({ root: '/home/vitorcds/servidor' } as SystemInfo);
    open(template({ volumes: [{ container: '/data' }, { container: '/config' }] }));
    form.name.set('smp');

    expect(form.ghostDirs().map((g) => g.path)).toEqual([
      '/home/vitorcds/servidor/smp',
      '/home/vitorcds/servidor/smp/data',
      '/home/vitorcds/servidor/smp/config',
    ]);
  });

  it('keeps the host port that was typed by hand', () => {
    open(template());
    form.setPortHost(form.ports()[0].id, '30000');

    expect(form.value().ports[0].host).toBe(30000);
  });

  it('opens a new port on the host under the same number', () => {
    open(template());
    form.addPort();
    form.setPortContainer(form.ports()[1].id, '8080');

    expect(form.ports()[1].host).toBe('8080');
  });

  it('stops following the container port once the host one is typed', () => {
    open(template());
    form.addPort();
    const id = form.ports()[1].id;
    form.setPortHost(id, '30000');
    form.setPortContainer(id, '8080');

    expect(form.ports()[1].host).toBe('30000');
  });

  it('names the folder of a volume that was added by hand', () => {
    open(template());
    form.addVolume();
    form.setVolumeContainer(form.volumes()[1].id, '/config');

    expect(form.volumes()[1].host).toBe('./config');
  });

  it('leaves out a row that was added and never filled', () => {
    open(template());
    form.addPort();
    form.addVolume();
    form.addField();

    expect(form.value().ports.length).toBe(1);
    expect(form.value().mounts.length).toBe(1);
    expect(form.value().extraEnv).toEqual({});
  });

  it('sends the port, the volume and the variable that were added', () => {
    open(template());
    form.name.set('smp');

    form.addPort();
    form.setPortContainer(form.ports()[1].id, '25575');
    form.setPortProtocol(form.ports()[1].id, 'udp');

    form.addVolume();
    form.setVolumeContainer(form.volumes()[1].id, '/config');

    form.addField();
    form.setFieldKey(form.fields()[0].id, 'TZ');
    form.setFieldValue(form.fields()[0].id, 'America/Sao_Paulo');

    expect(form.value().ports).toEqual([
      { host: 25565, container: 25565, protocol: 'tcp', label: 'game' },
      { host: 25575, container: 25575, protocol: 'udp', label: '' },
    ]);
    expect(form.value().mounts).toEqual([
      { host: './data', container: '/data' },
      { host: './config', container: '/config' },
    ]);
    expect(form.value().extraEnv).toEqual({ TZ: 'America/Sao_Paulo' });
  });
});

describe('InstanceForm: what an instance that already exists puts on the screen', () => {
  let fixture: ComponentFixture<InstanceForm>;
  let form: InstanceForm;
  let store: Store;

  function open(p: Template, seed: InstanceFormSeed): void {
    fixture.componentRef.setInput('template', p);
    fixture.componentRef.setInput('seed', seed);
    fixture.componentRef.setInput('mode', 'edit');
    fixture.detectChanges();
  }

  beforeEach(() => {
    localStorage.removeItem('okdock.locale');
    TestBed.configureTestingModule({
      providers: [provideHttpClient(), provideHttpClientTesting()],
    });
    fixture = TestBed.createComponent(InstanceForm);
    form = fixture.componentInstance;
    store = TestBed.inject(Store);
  });

  it('shows what is on disk, not what the template would offer', () => {
    const inst = instance('smp', 30000);
    inst.mounts = [{ host: '/containers/mundos', container: '/data' }];
    inst.memoryLimit = '6g';

    open(template(), seedOf(inst));

    expect(form.value().ports).toEqual([
      { host: 30000, container: 25565, protocol: 'tcp', label: 'game' },
    ]);
    expect(form.value().mounts).toEqual([{ host: '/containers/mundos', container: '/data' }]);
    expect(form.value().memoryLimit).toBe('6g');
  });

  it('does not bring back a port the instance does not publish', () => {
    const inst = instance('smp', 30000);
    inst.ports = [];

    open(template(), seedOf(inst));

    expect(form.value().ports).toEqual([]);
  });

  it('splits the variables the template declares from the ones typed by hand', () => {
    const inst = instance('smp', 25565);
    inst.env = { EULA: 'true', TZ: 'America/Sao_Paulo' };

    open(template({ fields: [{ key: 'EULA', label: 'EULA', type: 'bool' }] }), seedOf(inst));

    expect(form.value().values).toEqual({ EULA: 'true' });
    expect(form.value().extraEnv).toEqual({ TZ: 'America/Sao_Paulo' });
  });

  it('takes its own name back without calling it taken', () => {
    const inst = instance('smp', 25565);
    store.instances.set([inst]);

    open(template(), seedOf(inst));

    expect(form.nameError()).toBe('');
    expect(form.valid()).toBe(true);
  });

  it('refuses the name of another instance', () => {
    const inst = instance('smp', 25565);
    store.instances.set([inst, instance('familia', 25566)]);

    open(template(), seedOf(inst));
    form.name.set('familia');

    expect(form.nameError()).not.toBe('');
    expect(form.valid()).toBe(false);
  });

  it('leaves out of the budget what the instance already holds', () => {
    store.system.set({
      memoryBudget: 13 * 1024 ** 3,
      memoryCommitted: 12 * 1024 ** 3,
    } as SystemInfo);
    const inst = instance('smp', 25565);
    inst.memoryLimit = '8g';

    open(template(), seedOf(inst));
    fixture.componentRef.setInput('committed', '8g');

    expect(form.budgetWarning()).toBe('');
  });
});
