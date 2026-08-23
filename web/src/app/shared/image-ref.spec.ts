import { splitImage } from './image-ref';

describe('splitImage', () => {
  it('splits repository and tag', () => {
    expect(splitImage('itzg/minecraft-server:java21')).toEqual({
      repo: 'itzg/minecraft-server',
      tag: 'java21',
    });
  });

  it('returns an empty tag when there is none', () => {
    expect(splitImage('itzg/minecraft-server')).toEqual({
      repo: 'itzg/minecraft-server',
      tag: '',
    });
  });

  it('does not mistake the registry port for the tag', () => {
    expect(splitImage('registro.local:5000/terraria')).toEqual({
      repo: 'registro.local:5000/terraria',
      tag: '',
    });
  });
});
