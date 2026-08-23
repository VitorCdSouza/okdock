import { TestBed } from '@angular/core/testing';
import { provideHttpClient } from '@angular/common/http';
import { provideHttpClientTesting, HttpTestingController } from '@angular/common/http/testing';

import { Api, OkDockError } from './api';
import { I18n } from './i18n/i18n';

describe('Api: the error comes as a code, the sentence is built here', () => {
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

  it('uses the code and the params, not the API message', () => {
    const err = failStart({
      error: 'port_taken',
      message: 'port 25565/tcp already belongs to someone else',
      params: { port: 25565, proto: 'tcp', owner: 'outro' },
    });

    expect(err.code).toBe('port_taken');
    expect(err.message).toBe('port 25565/tcp already belongs to outro');
  });

  it('picks the variant by reason when there is one', () => {
    const err = failStart(
      {
        error: 'invalid_root',
        message: 'invalid root',
        params: { reason: 'not_dir', path: '/srv/games/arquivo' },
      },
      422,
    );

    expect(err.message).toBe('/srv/games/arquivo is not a directory');
  });

  it('falls back to the API text when the code is unknown', () => {
    const err = failStart({ error: 'something_new', message: 'a reason only the API knows' });

    expect(err.message).toBe('a reason only the API knows');
  });

  it('hands the structured problems to the form', () => {
    const err = failStart(
      {
        error: 'invalid_fields',
        message: 'some fields did not pass',
        problems: [{ field: 'MAX_PLAYERS', code: 'below_min', params: { min: 1 } }],
      },
      422,
    );

    expect(err.problems[0].field).toBe('MAX_PLAYERS');
    expect(TestBed.inject(I18n).problem(err.problems[0])).toBe('MAX_PLAYERS: the minimum is 1');
  });
});
