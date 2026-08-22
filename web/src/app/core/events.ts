import { DestroyRef, Injectable, NgZone, inject } from '@angular/core';
import { Observable } from 'rxjs';

import { ServerEvent } from './models';

@Injectable({ providedIn: 'root' })
export class Events {
  private readonly zone = inject(NgZone);
  private readonly destroyRef = inject(DestroyRef);

  stream(): Observable<ServerEvent> {
    return new Observable<ServerEvent>((subscriber) => {
      const source = new EventSource('/api/v1/events');

      const forward = (e: MessageEvent) => {
        try {
          this.zone.run(() => subscriber.next(JSON.parse(e.data) as ServerEvent));
        } catch {
        }
      };

      for (const type of ['instance.created', 'instance.changed', 'instance.deleted',
                          'instance.failed', 'instance.progress',
                          'instance.updated', 'instance.uptodate',
                          'dns.changed']) {
        source.addEventListener(type, forward as EventListener);
      }

      source.onerror = () => {
        if (source.readyState === EventSource.CLOSED) {
          this.zone.run(() => subscriber.error(new Error('stream de eventos caiu')));
        }
      };

      return () => source.close();
    });
  }

  logs(name: string, tail = 300): Observable<string> {
    return new Observable<string>((subscriber) => {
      const url = `/api/v1/instances/${encodeURIComponent(name)}/logs?tail=${tail}&follow=true`;
      const source = new EventSource(url);

      source.addEventListener('log', (e) => {
        const line = JSON.parse((e as MessageEvent).data) as string;
        this.zone.run(() => subscriber.next(line));
      });
      source.addEventListener('end', () => {
        this.zone.run(() => subscriber.complete());
      });
      source.onerror = () => {
        if (source.readyState === EventSource.CLOSED) {
          this.zone.run(() => subscriber.complete());
        }
      };

      return () => source.close();
    });
  }
}
