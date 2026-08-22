import { Injectable, computed, effect, signal } from '@angular/core';

import { MessageKey, pt } from './messages.pt';
import { en } from './messages.en';
import { ApiProblem } from '../models';
import { readSetting } from '../storage';

export type Locale = 'pt' | 'en';
export type LocalePref = 'auto' | Locale;

type PluralBase<K> = K extends `${infer Base}.one` ? Base : never;
export type PluralKey = PluralBase<MessageKey>;

const TABLES: Record<Locale, Record<MessageKey, string>> = { pt, en };
const KEY = 'okdock.locale';

@Injectable({ providedIn: 'root' })
export class I18n {
  readonly pref = signal<LocalePref>(loadPref());

  readonly locale = computed<Locale>(() => {
    const pref = this.pref();
    return pref === 'auto' ? detect() : pref;
  });

  private readonly table = computed(() => TABLES[this.locale()]);

  constructor() {
    effect(() => {
      const locale = this.locale();
      document.documentElement.lang = locale === 'pt' ? 'pt-BR' : 'en';
      try {
        localStorage.setItem(KEY, this.pref());
      } catch {
      }
    });
  }

  readonly t = (key: MessageKey, params?: Record<string, string | number>): string =>
    fill(this.table()[key], params);

  // chave montada em tempo de execucao, devolve undefined e quem chamou decide o que mostrar
  readonly maybe = (
    key: string,
    params?: Record<string, string | number>,
  ): string | undefined => {
    const text = (this.table() as Record<string, string | undefined>)[key];
    return text === undefined ? undefined : fill(text, params);
  };

  readonly problem = (p: ApiProblem): string => {
    const text = this.maybe(`problem.${p.code}`, p.params) ?? p.code;
    return `${p.field}: ${text}`;
  };

  readonly plural = (
    key: PluralKey,
    n: number,
    params?: Record<string, string | number>,
  ): string => {
    const variant = `${key}.${n === 1 ? 'one' : 'other'}` as MessageKey;
    return fill(this.table()[variant], { n, ...params });
  };

  readonly since = (iso: string | Date | undefined): string => {
    if (!iso) return '';
    const then = typeof iso === 'string' ? new Date(iso) : iso;
    const secs = Math.max(0, (Date.now() - then.getTime()) / 1000);
    if (secs < 60) return this.t('time.seconds');
    const mins = Math.floor(secs / 60);
    if (mins < 60) return this.t('time.minutes', { n: mins });
    const hours = Math.floor(mins / 60);
    if (hours < 24) return this.t('time.hours', { n: hours });
    const days = Math.floor(hours / 24);
    if (days < 30) return this.t('time.days', { n: days });
    const months = Math.floor(days / 30);
    if (months < 12) return this.plural('time.months', months);
    return this.plural('time.years', Math.floor(months / 12));
  };

  setPref(pref: LocalePref): void {
    this.pref.set(pref);
  }
}

function fill(text: string, params?: Record<string, string | number>): string {
  if (!params) return text;
  return text.replace(/\{(\w+)\}/g, (whole, name: string) =>
    name in params ? String(params[name]) : whole,
  );
}

function detect(): Locale {
  const tags = navigator.languages?.length ? navigator.languages : [navigator.language];
  return tags.some((tag) => tag?.toLowerCase().startsWith('pt')) ? 'pt' : 'en';
}

function loadPref(): LocalePref {
  const raw = readSetting(KEY);
  return raw === 'pt' || raw === 'en' || raw === 'auto' ? raw : 'auto';
}
