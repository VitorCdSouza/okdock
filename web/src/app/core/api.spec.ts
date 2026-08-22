import { TestBed } from '@angular/core/testing';
import { provideHttpClient } from '@angular/common/http';
import { provideHttpClientTesting, HttpTestingController } from '@angular/common/http/testing';

import { Api, OkDockError } from './api';
import { I18n } from './i18n/i18n';

describe('Api — erro vem em código, frase é montada aqui', () => {
  let api: Api;
  let http: HttpTestingController;

  beforeEach(() => {
    localStorage.removeItem('okdock.locale');
    TestBed.configureTestingModule({
      providers: [provideHttpClient(), provideHttpClientTesting()],
    });
    api = TestBed.inject(Api);
    http = TestBed.inject(HttpTestingController);
    TestBed.inject(I18n).setPref('en');
  });

  afterEach(() => localStorage.removeItem('okdock.locale'));

  function failStart(body: Record<string, unknown>, status = 409): OkDockError {
    let caught: OkDockError | undefined;
    api.start('smp').subscribe({ error: (e: OkDockError) => (caught = e) });
    http
      .expectOne('/api/v1/instances/smp/start')
      .flush(body as object, { status, statusText: 'Conflict' });
    return caught!;
  }

  it('usa o código e os params, não a mensagem da API', () => {
    const err = failStart({
      error: 'port_taken',
      message: 'a porta 25565/tcp já é de outro',
      params: { port: 25565, proto: 'tcp', owner: 'outro' },
    });

    expect(err.code).toBe('port_taken');
    expect(err.message).toBe('port 25565/tcp already belongs to outro');
  });

  it('escolhe a variante pelo reason quando existe', () => {
    const err = failStart(
      {
        error: 'invalid_root',
        message: 'raiz inválida',
        params: { reason: 'not_dir', path: '/srv/games/arquivo' },
      },
      422,
    );

    expect(err.message).toBe('/srv/games/arquivo is not a directory');
  });

  it('cai no texto da API quando o código é desconhecido', () => {
    const err = failStart({ error: 'algo_novo', message: 'motivo que só a API sabe' });

    expect(err.message).toBe('motivo que só a API sabe');
  });

  it('entrega os problemas estruturados para o formulário', () => {
    const err = failStart(
      {
        error: 'invalid_fields',
        message: 'alguns campos não passaram',
        problems: [{ field: 'MAX_PLAYERS', code: 'below_min', params: { min: 1 } }],
      },
      422,
    );

    expect(err.problems[0].field).toBe('MAX_PLAYERS');
    expect(TestBed.inject(I18n).problem(err.problems[0])).toBe('MAX_PLAYERS: the minimum is 1');
  });
});
