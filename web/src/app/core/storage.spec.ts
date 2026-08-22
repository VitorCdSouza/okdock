import { readSetting } from './storage';

describe('readSetting', () => {
  const key = 'okdock.locale';
  const legacy = 'gamedock.locale';

  beforeEach(() => {
    localStorage.removeItem(key);
    localStorage.removeItem(legacy);
  });

  afterEach(() => {
    localStorage.removeItem(key);
    localStorage.removeItem(legacy);
  });

  it('resgata o valor gravado com o nome antigo e migra a chave', () => {
    localStorage.setItem(legacy, 'en');

    expect(readSetting(key)).toBe('en');
    expect(localStorage.getItem(key)).withContext('não migrou para a chave nova').toBe('en');
    expect(localStorage.getItem(legacy)).withContext('a chave antiga ficou para trás').toBeNull();
  });

  it('a chave nova ganha da antiga', () => {
    localStorage.setItem(legacy, 'en');
    localStorage.setItem(key, 'pt');

    expect(readSetting(key)).toBe('pt');
    expect(localStorage.getItem(legacy)).withContext('não devia mexer na antiga').toBe('en');
  });

  it('devolve null quando nenhuma das duas existe', () => {
    expect(readSetting(key)).toBeNull();
  });
});
