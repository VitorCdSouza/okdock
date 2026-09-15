import { ComponentFixture, TestBed } from '@angular/core/testing';
import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';

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

function created(name: string): Instance {
  return {
    name,
    templateId: 'minecraft-java',
    category: 'games',
    image: 'itzg/minecraft-server:java21',
    env: {},
    ports: [{ host: 25565, container: 25565, protocol: 'tcp', label: 'game' }],
    mounts: [],
    memoryLimit: '4g',
    cpus: 2,
    restart: 'unless-stopped',
    stopGraceSeconds: 120,
    createdAt: '2026-08-26T12:00:00Z',
    updatedAt: '2026-08-26T12:00:00Z',
    dir: `/containers/${name}`,
    state: 'stopped',
  };
}

describe('NewInstance: what the two steps send', () => {
  let fixture: ComponentFixture<NewInstance>;
  let screen: NewInstance;
  let http: HttpTestingController;

  beforeEach(() => {
    localStorage.removeItem('okdock.locale');
    TestBed.configureTestingModule({
      providers: [provideHttpClient(), provideHttpClientTesting()],
    });
    fixture = TestBed.createComponent(NewInstance);
    screen = fixture.componentInstance;
    http = TestBed.inject(HttpTestingController);
    TestBed.inject(Store);
    fixture.detectChanges();
  });

  function fillIn(p: Template, name: string): void {
    screen.pick(p);
    screen.step.set(2);
    fixture.detectChanges();
    screen.form()!.name.set(name);
    fixture.detectChanges();
  }

  it('waits for a template before letting the first step end', () => {
    expect(screen.canAdvance()).toBe(false);

    screen.pick(template());

    expect(screen.canAdvance()).toBe(true);
  });

  it('waits for a name before letting the second step end', () => {
    screen.pick(template());
    screen.step.set(2);
    fixture.detectChanges();

    expect(screen.canAdvance()).toBe(false);

    screen.form()!.name.set('smp');
    fixture.detectChanges();

    expect(screen.canAdvance()).toBe(true);
  });

  it('keeps what was typed when the step goes back and forth', () => {
    fillIn(template(), 'smp');

    screen.back();
    fixture.detectChanges();
    screen.step.set(2);
    fixture.detectChanges();

    expect(screen.form()!.name()).toBe('smp');
  });

  it('sends the template, the ports and the volumes of the form', () => {
    fillIn(template(), 'smp');

    screen.next();

    const post = http.expectOne('/api/v1/instances');
    expect(post.request.body.name).toBe('smp');
    expect(post.request.body.templateId).toBe('minecraft-java');
    expect(post.request.body.ports).toEqual([
      { host: 25565, container: 25565, protocol: 'tcp', label: 'game' },
    ]);
    expect(post.request.body.mounts).toEqual([{ host: './data', container: '/data' }]);
    expect(post.request.body.start).toBe(true);
    post.flush(created('smp'));
  });

  it('leaves the instance down when the box is unchecked', () => {
    fillIn(template(), 'smp');
    screen.startAfterCreate.set(false);

    screen.next();

    const post = http.expectOne('/api/v1/instances');
    expect(post.request.body.start).toBe(false);
    post.flush(created('smp'));
  });
});
