import { Injectable, inject, signal } from '@angular/core';
import { HttpClient, HttpErrorResponse } from '@angular/common/http';
import { Observable, catchError, map, throwError } from 'rxjs';

import {
  ApiError,
  ApiProblem,
  ComposePreview,
  DnsStatus,
  Instance,
  InstanceDNS,
  InstancesResponse,
  Provider,
  SpecRequest,
  State,
  SystemInfo,
} from './models';
import { I18n } from './i18n/i18n';

const BASE = '/api/v1';

export class GameDockError extends Error {
  constructor(
    override readonly message: string,
    readonly code: string,
    readonly problems: ApiProblem[] = [],
    readonly status = 0,
  ) {
    super(message);
  }
}

@Injectable({ providedIn: 'root' })
export class Api {
  private readonly http = inject(HttpClient);
  private readonly i18n = inject(I18n);

  readonly lastError = signal<GameDockError | null>(null);

  system(): Observable<SystemInfo> {
    return this.get<SystemInfo>(`${BASE}/system`);
  }

  providers(): Observable<Provider[]> {
    return this.get<Provider[]>(`${BASE}/providers`);
  }

  instances(): Observable<{ instances: Instance[]; states: State[] }> {
    return this.get<InstancesResponse>(`${BASE}/instances`);
  }

  instance(name: string): Observable<Instance> {
    return this.get<Instance>(`${BASE}/instances/${encodeURIComponent(name)}`);
  }

  create(req: SpecRequest): Observable<Instance> {
    return this.post<Instance>(`${BASE}/instances`, req);
  }

  update(name: string, req: SpecRequest): Observable<Instance> {
    return this.wrap(
      this.http.put<Instance>(`${BASE}/instances/${encodeURIComponent(name)}`, req),
    );
  }

  previewCompose(req: SpecRequest): Observable<ComposePreview> {
    return this.post<ComposePreview>(`${BASE}/instances/preview-compose`, req);
  }

  compose(name: string): Observable<string> {
    return this.wrap(
      this.http.get(`${BASE}/instances/${encodeURIComponent(name)}/compose`, {
        responseType: 'text',
      }),
    );
  }

  start(name: string) {
    return this.action(name, 'start');
  }

  stop(name: string) {
    return this.action(name, 'stop');
  }

  restart(name: string) {
    return this.action(name, 'restart');
  }

  updateImage(name: string) {
    return this.action(name, 'update-image');
  }

  archive(name: string) {
    return this.action(name, 'archive');
  }

  unarchive(name: string) {
    return this.action(name, 'unarchive');
  }

  clearError(name: string) {
    return this.action(name, 'clear-error');
  }

  remove(name: string, keepData = true): Observable<void> {
    const url = `${BASE}/instances/${encodeURIComponent(name)}?keepData=${keepData}`;
    return this.wrap(this.http.delete<void>(url));
  }

  setRoot(root: string): Observable<SystemInfo> {
    return this.wrap(this.http.put<SystemInfo>(`${BASE}/system/root`, { root }));
  }

  dns(): Observable<DnsStatus> {
    return this.get<DnsStatus>(`${BASE}/dns`);
  }

  saveDnsToken(token: string): Observable<DnsStatus> {
    return this.wrap(this.http.put<DnsStatus>(`${BASE}/dns`, { token }));
  }

  addDnsDomain(domain: string): Observable<InstanceDNS> {
    return this.post<InstanceDNS>(`${BASE}/dns/domains`, { domain });
  }

  removeDnsDomain(domain: string): Observable<void> {
    return this.wrap(
      this.http.delete<void>(`${BASE}/dns/domains/${encodeURIComponent(domain)}`),
    );
  }

  linkDns(name: string, domain: string): Observable<InstanceDNS> {
    return this.wrap(
      this.http.put<InstanceDNS>(`${BASE}/instances/${encodeURIComponent(name)}/dns`, { domain }),
    );
  }

  unlinkDns(name: string): Observable<void> {
    return this.wrap(this.http.delete<void>(`${BASE}/instances/${encodeURIComponent(name)}/dns`));
  }

  syncDns(): Observable<void> {
    return this.post<void>(`${BASE}/dns/sync`, {});
  }

  private action(name: string, verb: string): Observable<void> {
    return this.post<void>(`${BASE}/instances/${encodeURIComponent(name)}/${verb}`, {});
  }

  private get<T>(url: string): Observable<T> {
    return this.wrap(this.http.get<T>(url));
  }

  private post<T>(url: string, body: unknown): Observable<T> {
    return this.wrap(this.http.post<T>(url, body));
  }

  private wrap<T>(source: Observable<T>): Observable<T> {
    return source.pipe(
      catchError((err: HttpErrorResponse) => {
        const parsed = this.toGameDockError(err);
        this.lastError.set(parsed);
        return throwError(() => parsed);
      }),
    );
  }

  // a API manda codigo e dados, e o texto do corpo e o ultimo recurso para um codigo novo
  private errorText(body: ApiError): string {
    const params = body.params ?? {};
    const reason = params['reason'];
    const specific = typeof reason === 'string' ? `error.${body.error}.${reason}` : '';
    return (
      (specific ? this.i18n.maybe(specific, params) : undefined) ??
      this.i18n.maybe(`error.${body.error}`, params) ??
      body.message
    );
  }

  private toGameDockError(err: HttpErrorResponse): GameDockError {
    if (err.status === 0) {
      return new GameDockError(this.i18n.t('api.offline'), 'offline', [], 0);
    }
    const body = err.error as ApiError | string | null;
    if (body && typeof body === 'object' && body.error) {
      return new GameDockError(
        this.errorText(body),
        body.error,
        body.problems ?? [],
        err.status,
      );
    }
    const fallback = err.message || this.i18n.t('api.httpError', { status: err.status });
    return new GameDockError(fallback, 'http', [], err.status);
  }
}
