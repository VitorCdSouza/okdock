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
    dir: '/srv/games/smp',
    state: 'stopped',
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

  it('traduz a etapa da operação que veio em código', () => {
    expect(card({ operation: { kind: 'provision', code: 'creating', message: '', startedAt: '' } }).opLabel())
      .toBe('criando container');
  });

  it('deixa passar intacta a linha crua do docker', () => {
    const line = 'Pulling from itzg/minecraft-server';

    expect(card({ operation: { kind: 'provision', message: line, startedAt: '' } }).opLabel()).toBe(line);
  });

  it('mostra o código da etapa quando a tela não conhece ele', () => {
    expect(card({ operation: { kind: 'provision', code: 'etapa_nova', message: '', startedAt: '' } }).opLabel())
      .toBe('etapa_nova');
  });

  it('cola o nome de DNS na porta para copiar', () => {
    const c = card({ dns: { domain: 'smp', hostname: 'smp.duckdns.org' } });

    expect(c.address()).toBe('smp.duckdns.org:25565');
  });

  it('não inventa endereço sem DNS vinculado', () => {
    expect(card().address()).toBe('');
  });

  it('diz há quanto tempo parou quando o docker não tem status', () => {
    expect(card({ state: 'stopped' }).meta()).toBe('parada há segundos');
    expect(card({ state: 'archived', archived: true }).meta()).toBe('arquivada há segundos');
  });

  it('sem status, o erro vira o código de saída', () => {
    expect(card({ state: 'error', exitCode: 137 }).meta()).toBe('saiu com código 137');
  });

  it('a ação do botão segue o estado', () => {
    expect(card({ state: 'stopped' }).action().verb).toBe('start');
    expect(card({ state: 'running' }).action().verb).toBe('stop');
    expect(card({ state: 'error' }).action().verb).toBe('fix');
    expect(card({ state: 'archived' }).action().verb).toBe('unarchive');
  });

  it('formata a RAM alocada e avisa quando não há teto', () => {
    expect(card().memoryAlloc()).toBe('4 GB');
    expect(card({ memoryLimit: '' }).memoryAlloc()).toBe('sem limite');
  });

  it('lista as portas com o protocolo só quando não é tcp', () => {
    const c = card({
      ports: [
        { host: 25565, container: 25565, protocol: 'tcp' },
        { host: 19132, container: 19132, protocol: 'udp' },
      ],
    });

    expect(c.portList()).toBe('25565, 19132/udp');
  });
});
