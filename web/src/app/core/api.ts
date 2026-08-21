import { Injectable, inject, signal } from '@angular/core';
import { HttpClient, HttpErrorResponse } from '@angular/common/http';
import { Observable, catchError, map, throwError } from 'rxjs';

import {
  ApiError,
  ComposePreview,
  Instance,
  InstancesResponse,
  Provider,
  SpecRequest,
  State,
  SystemInfo,
} from './models';

const BASE = '/api/v1';

export class GameDockError extends Error {
  constructor(
    override readonly message: string,
    readonly code: string,
    readonly problems: string[] = [],
    readonly status = 0,
  ) {
    super(message);
  }
}

@Injectable({ providedIn: 'root' })
export class Api {
  private readonly http = inject(HttpClient);

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
        const parsed = toGameDockError(err);
        this.lastError.set(parsed);
        return throwError(() => parsed);
      }),
    );
  }
}

function toGameDockError(err: HttpErrorResponse): GameDockError {
  if (err.status === 0) {
    return new GameDockError(
      'não consegui falar com a API do GameDock — ela está de pé?',
      'offline',
      [],
      0,
    );
  }
  const body = err.error as ApiError | string | null;
  if (body && typeof body === 'object' && body.message) {
    return new GameDockError(body.message, body.error, body.problems ?? [], err.status);
  }
  return new GameDockError(err.message || `erro ${err.status}`, 'http', [], err.status);
}
