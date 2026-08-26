import { TestBed } from '@angular/core/testing';
import { provideHttpClient } from '@angular/common/http';
import { provideHttpClientTesting } from '@angular/common/http/testing';

import { NewInstance } from './new-instance';
import { Store } from '../../core/state';
import { Instance, Template } from '../../core/models';

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
    dir: `/srv/games/${name}`,
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

  it('walks past a port another instance already holds', () => {
    store.instances.set([instance('smp', 25565)]);

    screen.pick(template());

    expect(screen.ports()[0].host).toBe(25566);
  });

  it('two ports of the same template do not land on each other', () => {
    store.instances.set([instance('smp', 25565)]);

    screen.pick(
      template({
        ports: [
          { container: 25565, protocol: 'tcp', label: 'game' },
          { container: 25565, protocol: 'udp', label: 'voice' },
          { container: 25566, protocol: 'tcp', label: 'rcon' },
        ],
      }),
    );

    // udp is a list of its own, tcp 25566 is the one that has to move
    expect(screen.ports().map((p) => p.host)).toEqual([25566, 25565, 25567]);
  });

  it('names the port and the owner when a typed port is taken', () => {
    store.instances.set([instance('smp', 25600)]);
    screen.pick(template());

    screen.setPort(25565, 'tcp', '25600');

    const warning = screen.portClash(25565, 'tcp');
    expect(warning).toContain('25600');
    expect(warning).toContain('smp');
  });

  it('says nothing while the port is free', () => {
    store.instances.set([instance('smp', 25600)]);
    screen.pick(template());

    expect(screen.portClash(25565, 'tcp')).toBe('');

    screen.setPort(25565, 'tcp', '25601');
    expect(screen.portClash(25565, 'tcp')).toBe('');
  });

  it('catches the same port asked for twice on this screen', () => {
    screen.pick(
      template({
        ports: [
          { container: 25565, protocol: 'tcp', label: 'game' },
          { container: 25575, protocol: 'tcp', label: 'rcon' },
        ],
      }),
    );

    screen.setPort(25575, 'tcp', '25565');

    expect(screen.portClash(25575, 'tcp')).toContain('25565');
    // the first line asked for it first, so it is the second one that complains
    expect(screen.portClash(25565, 'tcp')).toBe('');
  });

  it('keeps what was typed by hand', () => {
    screen.pick(template());
    screen.setPort(25565, 'tcp', '30000');

    expect(screen.ports()[0].host).toBe(30000);
  });
});
