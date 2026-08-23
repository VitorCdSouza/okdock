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

  it('rescues the value saved under the old name and migrates the key', () => {
    localStorage.setItem(legacy, 'en');

    expect(readSetting(key)).toBe('en');
    expect(localStorage.getItem(key)).withContext('did not migrate to the new key').toBe('en');
    expect(localStorage.getItem(legacy)).withContext('the old key was left behind').toBeNull();
  });

  it('the new key wins over the old one', () => {
    localStorage.setItem(legacy, 'en');
    localStorage.setItem(key, 'pt');

    expect(readSetting(key)).toBe('pt');
    expect(localStorage.getItem(legacy)).withContext('must not touch the old one').toBe('en');
  });

  it('returns null when neither exists', () => {
    expect(readSetting(key)).toBeNull();
  });
});
