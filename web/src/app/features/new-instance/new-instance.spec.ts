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

  it('keeps what was typed by hand', () => {
    screen.pick(template());
    screen.setPort(25565, 'tcp', '30000');

    expect(screen.ports()[0].host).toBe(30000);
  });
});
