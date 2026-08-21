# Catálogo de provedores

Um **provedor** é uma imagem Docker mais o schema dos campos que ela aceita. É
daqui que o wizard tira o formulário — o frontend não conhece nenhuma variável
de nenhum jogo.

Tudo vive em [`api/internal/catalog/providers.go`](../api/internal/catalog/providers.go).

## O que está no catálogo

| Jogo | Imagem | RAM padrão / mínima | Portas |
|---|---|---|---|
| Minecraft (Java) | `itzg/minecraft-server:java21` | 4g / 2g | 25565/tcp |
| Terraria | `ryshe/terraria:tshock-1.4.5.6-6.1.0` | 2g / 512m | 7777/tcp |
| Terraria (vanilla) | `ryshe/terraria:vanilla-1.4.5.7` | 2g / 512m | 7777/tcp |
| Imagem custom | qualquer | 2g / 256m | você define |

## As duas variantes de Terraria

O cliente do Terraria recusa entrar num servidor de versão diferente: *"You are
not using the same version as this server"*. Isso importa porque as duas
imagens andam em velocidades diferentes.

- **TShock** traz plugins, permissões e comandos de admin, mas leva semanas
  para acompanhar um lançamento do Terraria.
- **Vanilla** não tem nada disso e sai em dias.

Quando o jogo atualiza e o cliente para de conectar, a saída é a variante
vanilla — ou esperar o TShock.

Os dois provedores existem separados porque **não são a mesma imagem com outra
tag**: o `bootstrap.sh` é diferente. O da TShock aceita `WORLD_FILENAME` e cria
o mundo se `-autocreate` estiver nos argumentos. O da vanilla **sai com erro**
se `WORLD_FILENAME` estiver preenchido e o mundo não existir — lá o caminho do
mundo vai por `-world`, e `WORLD_FILENAME` fica vazio.

Trocar só a tag de uma instância TShock para vanilla, portanto, não funciona: o
container morre no boot e, com `restart: unless-stopped`, entra em crashloop. O
`ImagePattern` de cada provedor recusa essa combinação no momento de salvar,
com uma mensagem que diz o que fazer. Para migrar de variante, crie outra
instância apontando para a mesma pasta de mundo.

## ImagePattern

Cada provedor de jogo declara quais imagens sabe configurar, como expressão
regular:

| Provedor | Padrão |
|---|---|
| Minecraft (Java) | `^itzg/minecraft-server(:\|$)` |
| Terraria | `^ryshe/terraria:tshock-` |
| Terraria (vanilla) | `^ryshe/terraria:vanilla-` |
| Imagem custom | vazio — aceita qualquer uma |

O padrão precisa ser largo o bastante para deixar trocar de versão (é assim que
se atualiza uma instância) e estreito o bastante para barrar outra variante. Os
testes cobrem os dois lados, e checam que nenhum padrão rejeita a própria
imagem padrão do provedor.

## Tags fixas

As imagens de jogo usam tag de versão, nunca `:latest`. Tag móvel troca a
versão do servidor sozinha no próximo recreate, e o sintoma aparece longe da
causa: o jogador é quem descobre, ao não conseguir entrar. `TestGameImagesArePinned`
falha se alguém reintroduzir uma.

Atualizar de versão é trocar o campo **Imagem** da instância no painel e salvar
— o mundo nos volumes é preservado.

O provedor `custom` é o único que aceita variáveis fora do schema — é a saída
para uma imagem que o catálogo ainda não descreve.

## Estado de verificação dos schemas

Os dois provedores de jogo foram exercitados contra um container de verdade em
21/08/2026: criar instância pelo painel, subir, e confirmar que a configuração
pegou. Minecraft gera o mundo e aceita conexão; Terraria idem, com
`gamedock.wld` criado pelo `-autocreate`.

O catálogo já teve outros seis jogos (Minecraft Bedrock, Palworld, Valheim,
ARK, Factorio, Satisfactory), removidos em 21/08/2026 a pedido do usuário. Os
schemas deles estão no histórico do git, em
`internal/catalog/providers.go` antes do commit que os retirou — vale
recuperá-los de lá em vez de reescrever do zero, mas **sem confiar neles**: só
Minecraft e Terraria foram executados, e o caso do Terraria mostrou que o erro
pode não ser um nome trocado e sim o mecanismo inteiro estar errado.

Ao voltar com um provedor, o roteiro que funcionou foi:

1. `skopeo inspect --config docker://<imagem>` — mostra entrypoint e variáveis
   declaradas sem baixar a imagem.
2. Ler o entrypoint (`docker run --rm --entrypoint cat <imagem> /caminho/do/script`)
   — é o que revela se a imagem lê configuração do ambiente ou de argumentos.
3. Subir uma instância e confirmar que a configuração surtiu efeito.

O passo 1 sozinho engana: **a lista de variáveis declaradas não é a lista
completa de variáveis lidas.** Muitas imagens usam `${VAR:-default}` no script
sem declarar nada no `ENV`. A inspeção prova o que existe, nunca o que não
existe.

Campo com nome errado não quebra o `up`: a variável é ignorada e o efeito é a
configuração não pegar. Configuração que a imagem espera como **argumento** é
pior: o sintoma é o container subir e não fazer nada, como aconteceu com
Terraria antes do `TargetArg`.

Nada disso impede usar um jogo fora do catálogo — o provedor `custom` sempre
aceita a imagem com as variáveis digitadas à mão.

## Adicionando um provedor

1. Acrescente uma entrada em `providers` no `providers.go`.
2. `go test ./internal/catalog/` — `TestAllProvidersAreUsable` já cobre o
   básico: identificação completa, volume de mundo marcado, chaves sem
   repetição, enum com opções, e **todo default passando na própria
   validação** (um default fora do schema faria a instância nascer inválida sem
   o usuário ter tocado em nada).
3. Nada muda no frontend.

### Campo que vira argumento, e não variável

`Target: TargetArg` mais `Flag: "-autocreate"` manda o valor para o `command:`
do serviço em vez do ambiente. Campo booleano emite só a flag quando
verdadeiro, sem valor. A ordem dos argumentos segue a ordem dos campos no
provedor, e não a do mapa, para o compose gerado não mudar a cada renderização.

Campo secreto que é argumento vira `${CHAVE}` no compose, e o valor vai para o
`.env` — o `docker compose` interpola lendo o `.env` do diretório do projeto.
Assim a senha não aparece no YAML nem quando é passada na linha de comando.

Os campos que merecem atenção:

- **`secret: true`** em qualquer senha. É o que mantém o valor fora do
  `docker-compose.yml`.
- **`minMemory`** honesto. É o que o painel usa para recusar uma instância que
  morreria com `Exited (137)`.
- **`stopGraceSeconds`** generoso em jogo que salva ao desligar. Minecraft e
  ARK corrompem save se levarem SIGKILL no meio da gravação; 120 s e 180 s são
  os valores usados hoje.
- **`optional: true`** em porta de RCON e de query. Elas não são publicadas por
  padrão — publicar RCON sem necessidade é abrir um console remoto do servidor.
- **`advanced: true`** no que quase ninguém mexe, para o formulário não
  assustar.

## Como a validação funciona

`Provider.Validate` aplica os defaults, confere tipo, faixa e enum, e junta
**todos** os problemas antes de devolver — o formulário marca os campos errados
de uma vez em vez de obrigar a salvar várias vezes para descobrir o resto.
