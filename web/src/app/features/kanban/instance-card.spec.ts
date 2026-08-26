import { TestBed } from '@angular/core/testing';

import { InstanceCard } from './instance-card';
import { I18n } from '../../core/i18n/i18n';
import { Instance } from '../../core/models';

function instance(over: Partial<Instance> = {}): Instance {
  return {
    name: 'smp',
    templateId: 'minecraft-java',
    category: 'games',
    image: 'itzg/minecraft-server:java21',
    env: {},
    ports: [{ host: 25565, container: 25565, protocol: 'tcp' }],
    mounts: [],
    memoryLimit: '4g',
    cpus: 2,
    restart: 'unless-stopped',
    stopGraceSeconds: 120,
    createdAt: '2026-08-21T00:00:00Z',
    updatedAt: new Date().toISOString(),
    dir: '/containers/smp',
    state: 'stopped',
    editable: true,
    ...over,
  };
}

describe('InstanceCard', () => {
  function card(over: Partial<Instance> = {}): InstanceCard {
    const fixture = TestBed.createComponent(InstanceCard);
    fixture.componentRef.setInput('instance', instance(over));
    return fixture.componentInstance;
  }

  beforeEach(() => {
    localStorage.removeItem('okdock.locale');
    TestBed.configureTestingModule({});
    TestBed.inject(I18n).setPref('pt');
  });

  afterEach(() => localStorage.removeItem('okdock.locale'));

  it('does not break on an external container, which comes with no ports or volumes', () => {
    const external = card({
      external: true,
      project: 'media',
      ports: null as never,
      mounts: null as never,
      memoryLimit: '',
      cpus: 0,
    });

    expect(external.portList()).toBe('-');
    expect(external.portCount()).toBe(0);
    expect(external.address()).toBe('');
  });

  it('an external container with no readable compose only reaches the console', () => {
    const fixture = TestBed.createComponent(InstanceCard);
    fixture.componentRef.setInput(
      'instance',
      instance({ external: true, editable: false, project: 'media', state: 'running' }),
    );
    fixture.componentInstance.menuOpen.set(true);
    fixture.detectChanges();

    const itens: HTMLElement[] = Array.from(fixture.nativeElement.querySelectorAll('.menu button'));

    expect(itens.map((b) => b.textContent!.trim())).toEqual(['Detalhes']);
  });

  it('an external container whose compose was read is edited, but still not deleted', () => {
    const fixture = TestBed.createComponent(InstanceCard);
    fixture.componentRef.setInput(
      'instance',
      instance({ external: true, editable: true, project: 'media', state: 'running' }),
    );
    fixture.componentInstance.menuOpen.set(true);
    fixture.detectChanges();

    const itens: HTMLElement[] = Array.from(fixture.nativeElement.querySelectorAll('.menu button'));

    expect(itens.map((b) => b.textContent!.trim())).toEqual(['Editar']);
  });

  it('the project chip only shows what the name does not already say', () => {
    expect(card({ external: true, project: 'bottelegram' }).showProject()).toBeTrue();
    // the project repeating the container name is noise
    expect(card({ external: true, project: 'smp' }).showProject()).toBeFalse();
    // and inside the stack tile the project is already the title
    const fixture = TestBed.createComponent(InstanceCard);
    fixture.componentRef.setInput('instance', instance({ external: true, project: 'media' }));
    fixture.componentRef.setInput('inStack', true);
    expect(fixture.componentInstance.showProject()).toBeFalse();
  });

  it('a panel instance keeps edit and delete', () => {
    const fixture = TestBed.createComponent(InstanceCard);
    fixture.componentRef.setInput('instance', instance());
    fixture.componentInstance.menuOpen.set(true);
    fixture.detectChanges();

    const itens: HTMLElement[] = Array.from(fixture.nativeElement.querySelectorAll('.menu button'));

    expect(itens.map((b) => b.textContent!.trim())).toEqual(['Editar', 'Excluir']);
  });

  it('translates the operation step that came as a code', () => {
    expect(card({ operation: { kind: 'provision', code: 'creating', message: '', startedAt: '' } }).opLabel())
      .toBe('criando container');
  });

  it('lets the raw docker line through untouched', () => {
    const line = 'Pulling from itzg/minecraft-server';

    expect(card({ operation: { kind: 'provision', message: line, startedAt: '' } }).opLabel()).toBe(line);
  });

  it('shows the step code when the screen does not know it', () => {
    expect(card({ operation: { kind: 'provision', code: 'new_step', message: '', startedAt: '' } }).opLabel())
      .toBe('new_step');
  });

  it('glues the DNS name to the port for copying', () => {
    const c = card({ dns: { domain: 'smp', hostname: 'smp.duckdns.org' } });

    expect(c.address()).toBe('smp.duckdns.org:25565');
  });

  it('invents no address without a linked DNS', () => {
    expect(card().address()).toBe('');
  });

  it('says how long it has been stopped when docker has no status', () => {
    expect(card({ state: 'stopped' }).meta()).toBe('parada há segundos');
    expect(card({ state: 'archived', archived: true }).meta()).toBe('arquivada há segundos');
  });

  it('with no status, the error becomes the exit code', () => {
    expect(card({ state: 'error', exitCode: 137 }).meta()).toBe('saiu com código 137');
  });

  it('the button action follows the state', () => {
    expect(card({ state: 'stopped' }).action().verb).toBe('start');
    expect(card({ state: 'running' }).action().verb).toBe('stop');
    expect(card({ state: 'error' }).action().verb).toBe('fix');
    expect(card({ state: 'archived' }).action().verb).toBe('unarchive');
  });

  it('formats the allocated RAM and says when there is no cap', () => {
    expect(card().memoryAlloc()).toBe('4 GB');
    expect(card({ memoryLimit: '' }).memoryAlloc()).toBe('sem limite');
  });

  it('lists ports with the protocol only when it is not tcp', () => {
    const c = card({
      ports: [
        { host: 25565, container: 25565, protocol: 'tcp' },
        { host: 19132, container: 19132, protocol: 'udp' },
      ],
    });

    expect(c.portList()).toBe('25565, 19132/udp');
  });
});
