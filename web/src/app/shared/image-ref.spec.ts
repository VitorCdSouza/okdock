import { splitImage } from './image-ref';

describe('splitImage', () => {
  it('separa repositório e etiqueta', () => {
    expect(splitImage('itzg/minecraft-server:java21')).toEqual({
      repo: 'itzg/minecraft-server',
      tag: 'java21',
    });
  });

  it('devolve etiqueta vazia quando não há', () => {
    expect(splitImage('itzg/minecraft-server')).toEqual({
      repo: 'itzg/minecraft-server',
      tag: '',
    });
  });

  it('não confunde a porta do registro com a etiqueta', () => {
    expect(splitImage('registro.local:5000/terraria')).toEqual({
      repo: 'registro.local:5000/terraria',
      tag: '',
    });
  });
});
