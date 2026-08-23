# OkDock

Painel para criar e administrar servidores de jogo em Docker num servidor
caseiro. Cada instância vira um diretório com `docker-compose.yml` próprio, e o
painel roda `docker compose` em cima dele — nada é criado pela SDK do Docker.
Isso é de propósito: se o painel cair, tudo continua administrável pelo
terminal.

```
okdock/
├── api/     API em Go — templates, geração de compose, orquestração
├── web/     painel em Angular 20
├── docs/    arquitetura, contrato da API, templates
└── Dockerfile, docker-compose.yml, Makefile
```

## Por que monorepo com uma branch só

`api/` e `web/` compartilham um contrato. Uma branch por app impediria o commit
que muda a resposta da API e o cliente que a consome ao mesmo tempo — viraria
sempre dois commits que nunca se encontram, e um merge entre eles não faria
sentido. Com pastas na mesma branch a mudança é atômica, e os path filters do
CI garantem que mexer no Angular não roda o CI do Go.

## Rodando em desenvolvimento

```bash
make dev
```

Sobe a API em `:8080` e o Angular em `:4200` no mesmo terminal, com a saída dos
dois prefixada por `[api]` e `[web]`. Ctrl-C derruba os dois. Abra
**http://localhost:4200** — use `localhost`, e não `127.0.0.1`: o `ng serve`
escuta só em IPv6.

Se preferir um terminal para cada, `make dev-api` e `make dev-web` continuam lá.

A API grava as instâncias em `./.data` por padrão nesse modo, então dá para
mexer sem tocar em `/srv/games`.

Nada disso exige Docker instalado na máquina de desenvolvimento: sem daemon o
painel abre, lista vazio e avisa que o docker não respondeu. Os testes também
passam sem daemon — o runner é uma interface com um `Fake`.

```bash
make test    # go test ./... + testes do Angular
make lint    # go vet + gofmt
```

## Rodando no servidor

```bash
docker compose up -d --build
```

O painel sobe em `:8080` com o frontend embutido no próprio binário — um
container só, sem servidor web separado.

Duas coisas no `docker-compose.yml` de deploy não são detalhe:

- **`/var/run/docker.sock` montado.** É a única via até o Docker; o painel não
  fala TCP nem SSH.
- **`/srv/games:/srv/games`, com os dois lados iguais.** Os bind mounts do
  compose gerado são resolvidos pelo daemon do *host*, que não enxerga o
  sistema de arquivos do container. Se o caminho divergir, o mundo é criado no
  lugar errado, silenciosamente.

O container leva o próprio `docker compose` (plugin do Alpine), então não
depende da versão instalada no host.

### Variáveis

| Variável | Padrão | Para quê |
|---|---|---|
| `OKDOCK_ADDR` | `:8080` | endereço de escuta |
| `OKDOCK_ROOT` | `/srv/games` | raiz inicial das instâncias e casa do `.okdock/` |
| `OKDOCK_MEMORY_RESERVE` | `2g` | RAM que fica fora do orçamento das instâncias |
| `OKDOCK_ALLOW_ORIGIN` | vazio | libera CORS; só para o `ng serve` |
| `OKDOCK_LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |

## O que o painel faz

- **Kanban por estado** — parado, provisionando, iniciando, rodando,
  atualizando, erro, arquivado. Atualiza sozinho por SSE.
- **Templates por categoria** — jogos, mídia, banco de dados, rede,
  utilidades. Quatro vêm prontos; os outros você cadastra pelo botão **Novo
  template**, informando imagem, portas, volumes e os campos de configuração.
  Editar um template pronto grava uma cópia sua, e apagar a cópia devolve o
  original.
- **Wizard de nova instância** — no `＋` da coluna PARADO. Os campos do
  formulário vêm do template escolhido, não são fixos no frontend. O
  repositório da imagem é o do template e só a etiqueta se escolhe, `latest`
  inclusive. O `docker-compose.yml` gerado
  fica na aba **compose.yml** da instância. Um dos nomes de DNS já cadastrados
  nas configurações dá para vincular ali mesmo, sem passar pela tela da
  instância.
- **Orçamento de RAM** — o painel soma o teto de memória das instâncias de pé
  antes de deixar subir mais uma. É o que evita descobrir o limite pelo
  `Exited (137)`.
- **Nome fixo para convidar** — vincula um subdomínio do
  [duckdns.org](https://www.duckdns.org) à instância e mantém o IP em dia
  sozinho, para o card mostrar `smp.duckdns.org:25565` pronto para copiar. O
  subdomínio tem que existir na conta antes: a API do duckdns não cria nome, só
  atualiza IP. E ela não conhece porta — encaminhar a porta no roteador
  continua sendo trabalho manual.
- **Console ao vivo** e leitura do `docker-compose.yml` como está no disco.
- **Configurações** (a engrenagem) — raiz das instâncias, versão do Docker,
  token do duckdns com a lista de nomes da conta (cada um com o IP que o
  serviço confirmou), quais números aparecem na barra de cima e o idioma da
  interface (português, inglês ou o do navegador).

A escolha de idioma e a de quais números aparecem na barra ficam no
`localStorage` do navegador, não no servidor: sem login, uma preferência
gravada lá valeria para todo mundo da casa. A API não manda frase pronta para a
tela: manda código e dados (`port_taken` com porta e dono, `below_min` com o
mínimo), e o painel escreve no idioma escolhido. Só o que o docker escreve —
linha de log, status do container — aparece como veio.

Senha nunca entra no `docker-compose.yml`: os campos marcados como secretos vão
para um `.env` ao lado, com permissão `0600` e fora do controle de versão.

## Um diretório de instância

```
/srv/games/smp-familia/
├── docker-compose.yml    gerado; é o que o Docker lê
├── .env                  só os segredos; 0600; nunca versionado
├── .okdock.json          de qual template isto veio
└── data/                 o mundo
```

A configuração do painel — o token do duckdns, os vínculos de domínio e a raiz
escolhida na tela de configurações — fica fora disso, em `/srv/games/.okdock/`
com `0600`. O ponto no nome não é enfeite: é o que impede a pasta de ser lida
como instância.

Essa pasta fica sempre na raiz com que o processo subiu (`OKDOCK_ROOT`),
mesmo depois de trocar a raiz das instâncias pelo painel — é ela que guarda
para onde a raiz mudou. Trocar a raiz não move o que já existe: o docker guarda
o caminho absoluto dos bind mounts, então as instâncias antigas continuam de pé
onde estão e voltam à lista se a raiz voltar. Dentro do container, só vale
caminho que esteja montado lá com o mesmo nome de fora.

O nome do diretório, o `name:` do projeto no compose e o nome do container são
sempre o mesmo texto. Quando divergem, `docker compose` age sobre um projeto
diferente do que a pasta sugere.

## Documentação

- [`docs/arquitetura.md`](docs/arquitetura.md) — como as camadas se encaixam e
  por quê
- [`docs/api.md`](docs/api.md) — contrato REST + SSE
- [`docs/templates.md`](docs/templates.md) — templates prontos e como escrever
  outro

## Vindo do GameDock

O projeto se chamava GameDock. O painel continua lendo o que ficou com o nome
antigo, então uma instalação existente sobe sem passo manual:

| Antes | Agora | Como é lido |
|---|---|---|
| `GAMEDOCK_*` | `OKDOCK_*` | a variável antiga vale quando a nova não está definida, com aviso no log |
| `<raiz>/.gamedock/` | `<raiz>/.okdock/` | a pasta antiga é lida quando a nova não existe |
| `.gamedock.json` na instância | `.okdock.json` | o arquivo antigo é lido, e a primeira gravação troca pelo novo |
| `gamedock.locale`, `gamedock.metrics` | `okdock.*` | a chave antiga do navegador migra na primeira leitura |

O que **não** se renomeia sozinho: o diretório e o `container_name` das
instâncias já criadas continuam com o texto que você deu a elas, e o label
`gamedock.managed` fica nos containers antigos até a próxima recriação. Nenhum
dos dois muda comportamento — o painel encontra a instância pelo diretório.

## Melhorias planejadas

Lista de trabalho anotada, ainda não implementada.

- Suporte a mods: descobrir de alguma forma se a imagem aceita mods e, quando
  aceitar, mostrar uma aba **Mods** onde se arrasta arquivos soltos ou um
  `.zip`.
- Largura das colunas do quadro: com doze containers de pé, RODANDO fica
  espremida e o scroll vertical vira uma coluna sem fim, enquanto PARADO fica
  vazia ao lado. A coluna devia crescer com o que tem dentro — hoje
  `grid-auto-columns: minmax(252px, 1fr)` em `web/src/app/features/kanban/kanban.css`
  dá o mesmo tamanho para todas.
- A coluna ERRO só aparece rolando o quadro no eixo X. Com cinco colunas fixas
  o quadro passou da largura da tela, e a última fica escondida sem nada
  indicando que existe.
- Melhorar a categoria dos containers: a tabela de palpites em
  `api/internal/template/guess.go` acerta o óbvio e erra o resto — no servidor,
  `flaresolverr` caiu em mídia e `telegramPromoBot` em outros. Ver se dá para
  usar mais do que o nome da imagem (labels, portas, imagem base) antes de
  ampliar a lista de palavras.
- Descobrir por que não dá para mexer nos containers que já existiam. Parar,
  subir e reiniciar deveriam funcionar (`ContainerAction` no `dockerx`, com
  `docker stop/start/restart`), e a API responde 202 nos testes — falta
  conferir no servidor se o erro vem do docker, do card (que talvez nem mostre
  o botão) ou do 409 `external_instance`, que é recusa proposital de atualizar
  imagem, arquivar e excluir, mas hoje aparece igual a uma falha.
