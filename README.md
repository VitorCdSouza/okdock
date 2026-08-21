# GameDock

Painel para criar e administrar servidores de jogo em Docker num servidor
caseiro. Cada instância vira um diretório com `docker-compose.yml` próprio, e o
painel roda `docker compose` em cima dele — nada é criado pela SDK do Docker.
Isso é de propósito: se o painel cair, tudo continua administrável pelo
terminal.

```
gamedock/
├── api/     API em Go — catálogo, geração de compose, orquestração
├── web/     painel em Angular 20
├── docs/    arquitetura, contrato da API, catálogo
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
| `GAMEDOCK_ADDR` | `:8080` | endereço de escuta |
| `GAMEDOCK_ROOT` | `/srv/games` | raiz dos diretórios de instância |
| `GAMEDOCK_MEMORY_RESERVE` | `2g` | RAM que fica fora do orçamento das instâncias |
| `GAMEDOCK_ALLOW_ORIGIN` | vazio | libera CORS; só para o `ng serve` |
| `GAMEDOCK_LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |

## O que o painel faz

- **Kanban por estado** — parado, provisionando, iniciando, rodando,
  atualizando, erro, arquivado. Atualiza sozinho por SSE.
- **Wizard de nova instância** — os campos do formulário vêm do provedor da
  imagem, não são fixos no frontend. O compose é mostrado antes de gravar
  qualquer coisa.
- **Orçamento de RAM** — o painel soma o teto de memória das instâncias de pé
  antes de deixar subir mais uma. É o que evita descobrir o limite pelo
  `Exited (137)`.
- **Console ao vivo** e leitura do `docker-compose.yml` como está no disco.

Senha nunca entra no `docker-compose.yml`: os campos marcados como secretos vão
para um `.env` ao lado, com permissão `0600` e fora do controle de versão.

## Um diretório de instância

```
/srv/games/smp-familia/
├── docker-compose.yml    gerado; é o que o Docker lê
├── .env                  só os segredos; 0600; nunca versionado
├── .gamedock.json        de qual provedor do catálogo isto veio
└── data/                 o mundo
```

O nome do diretório, o `name:` do projeto no compose e o nome do container são
sempre o mesmo texto. Quando divergem, `docker compose` age sobre um projeto
diferente do que a pasta sugere.

## Documentação

- [`docs/arquitetura.md`](docs/arquitetura.md) — como as camadas se encaixam e
  por quê
- [`docs/api.md`](docs/api.md) — contrato REST + SSE
- [`docs/catalogo.md`](docs/catalogo.md) — provedores suportados e como
  adicionar outro
