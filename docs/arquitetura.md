# Arquitetura

## As camadas da API

```
cmd/gamedock          flags, logger, servidor HTTP, desligamento gracioso
  └── httpapi         REST + SSE. Só traduz HTTP; não conhece Docker.
        └── manager   orquestração: orçamento de RAM, conflito de porta,
                      operações em background, derivação de estado
              ├── store     persistência em disco (compose, .env, metadados)
              │     └── compose   geração do YAML e do .env
              ├── dockerx   `docker compose` atrás de uma interface
              ├── catalog   provedores e schema de campos
              └── system    RAM, CPU e disco do host
```

A regra é uma só: nada acima de `manager` conhece Docker, e nada abaixo dele
conhece HTTP. `httpapi` traduz erro de domínio em status:

| Erro | Status |
|---|---|
| `store.ErrNotFound` | 404 |
| `store.ErrExists` | 409 |
| `catalog.ValidationError` | 422, com a lista de campos |
| `manager.ErrBudget`, `ErrPortTaken` | 409 |
| `dockerx.Error` | 409 — o docker falhou por algo que o usuário resolve |

## Decisões que não são acidente

### O compose em disco é a fonte da verdade

O painel não cria container pela SDK. Ele escreve um `docker-compose.yml` e
chama o binário do `docker compose`. Custa um processo por operação, e em troca
o servidor continua administrável pelo terminal se o painel morrer — que é o
cenário provável num servidor caseiro.

`.gamedock.json` guarda só o que o compose não sabe dizer: de qual provedor do
catálogo aquilo saiu. Se o arquivo sumir, a instância continua subindo pelo
terminal; só deixa de ser editável pelo formulário.

### O nome é um só

Nome do diretório, `name:` do projeto compose e `container_name` são o mesmo
texto, e `store.Get` reescreve o nome a partir do diretório. Sem `name:`
explícito o projeto herda o nome da pasta, e quando os dois divergem
`docker compose` age sobre um projeto diferente do que a pasta sugere.

### Segredo nunca no compose

Campos marcados `secret: true` no catálogo não entram no YAML: vão para um
`.env` com `0600`, referenciado por `env_file`. A lista de chaves secretas fica
gravada na Spec da instância, não é relida do catálogo — se o schema do provedor
mudar depois, uma senha já gravada não passa a vazar.

Os campos não secretos ficam inline em `environment:`, porque poder ler a
configuração no terminal é metade do motivo de gerar compose.

### O orçamento de RAM é conferido antes

`manager.checkBudget` soma o teto de memória das instâncias **de pé** e compara
com `RAM total − reserva` antes de qualquer `up`. Sem isso o limite só aparece
como `Exited (137)`, depois de o kernel matar o processo.

Instância parada não conta: ela não ocupa nada.

### Operações longas rodam soltas

`pull` e `up` levam minutos. Rodam em goroutine com `context.Background()`, não
com o contexto do request — fechar a aba do navegador não pode abortar um
download de 2 GB. O andamento vive num registro de operações em memória, e é o
que enche a barra de progresso do card.

Consequência aceita: reiniciar o painel esquece as operações em andamento. O
estado real volta na próxima leitura do `docker compose ps`.

### SSE avisa, não carrega dado

O stream de eventos manda `{type, instance}` e nada mais. O frontend recarrega
a lista. Isso mantém a API como fonte única da verdade — reconciliar mutação
parcial no cliente é como aparece card fantasma no kanban. O `Store` do Angular
agrupa os eventos numa janela de 250 ms, porque um `docker pull` emite dezenas
de linhas por segundo.

### O runner do Docker é uma interface

A máquina de desenvolvimento não tem daemon. `dockerx.Runner` tem duas
implementações: `CLI`, que chama o binário, e `Fake`, em memória. Todo o teste
de `manager` roda no `Fake`, então `go test ./...` passa sem Docker.

`dockerx.Error` carrega o stderr do comando — sem ele o erro seria
`exit status 1`, enquanto o motivo real (`port is already allocated`,
`no space left on device`) fica no stderr.

## O frontend

Angular 20 standalone, com signals. Sem NgRx: o estado cabe num serviço
(`core/state.ts`) com quatro signals e alguns `computed`.

`shared/provider-form` monta o formulário a partir do schema que a API mandou.
O frontend não conhece nenhuma variável de nenhum jogo — adicionar um jogo novo
é mexer só no `catalog` do Go.

Em produção o binário Go serve o `dist/` do Angular (via `go:embed`) e a API na
mesma origem: um container, uma porta, sem CORS. Em desenvolvimento o
`ng serve` faz proxy de `/api` para `localhost:8080`.

## O que ainda não existe

- **Autenticação.** O painel controla o `docker.sock`; quem alcança a porta
  8080 controla o servidor. Hoje isso depende de ele não estar exposto para
  fora da LAN. Antes de qualquer exposição, precisa de login.
- **Upload de mundo e navegador de arquivos** — a aba existe no design.
- **Backups** — a aba existe no design.
- **Console interativo** — hoje o log é só leitura; falta mandar comando (RCON
  ou `docker attach`).
- **Reconstrução do estado de operações** após reinício do painel.
