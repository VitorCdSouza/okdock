# Contrato da API

Base: `/api/v1`. Tudo JSON, exceto onde indicado. Sem autenticação — ver a
seção final de `docs/arquitetura.md`.

## Erros

Todo erro devolve o mesmo corpo:

```json
{
  "error": "memory_budget",
  "message": "smp pede 8g, mas só há 3 GB livres no orçamento de 13 GB (as instâncias de pé já usam 10 GB)",
  "problems": ["MAX_PLAYERS: máximo é 200"]
}
```

`message` é escrito para aparecer na tela como está. `problems` só vem no 422.

| `error` | Status | Quando |
|---|---|---|
| `not_found` | 404 | instância ou provedor inexistente |
| `already_exists` | 409 | já existe instância com esse nome |
| `invalid_fields` | 422 | valores fora do schema do provedor |
| `memory_budget` | 409 | não cabe na RAM do host |
| `port_taken` | 409 | porta já usada por outra instância |
| `docker_failed` | 409 | o `docker compose` falhou; `message` traz o stderr |
| `bad_request` | 400 | corpo malformado ou campo desconhecido |

Campo desconhecido no corpo é rejeitado de propósito: costuma ser erro de
digitação do cliente, e aceitar em silêncio faz a configuração sair diferente
do que o usuário pediu.

## Sistema

### `GET /system`

```json
{
  "memoryTotal": 16106127360, "memoryAvailable": 9663676416, "memoryUsed": 6442450944,
  "diskTotal": 987842478080, "diskFree": 646392975360, "diskUsed": 341449502720,
  "cpuCount": 8, "cpuPercent": 41.2,
  "dockerVersion": "27.1.0",
  "memoryReserve": 2147483648,
  "memoryBudget": 13958643712,
  "memoryCommitted": 6442450944,
  "memoryPlanned": 12884901888,
  "instanceCount": 4
}
```

`memoryBudget` é `memoryTotal − memoryReserve`. `memoryCommitted` soma o teto
das instâncias de pé; `memoryPlanned` soma também as paradas. Sem daemon, vem
`dockerError` no lugar de `dockerVersion`.

### `GET /health`

`{"status":"ok"}`. Não toca no Docker — é o healthcheck do container.

## Catálogo

### `GET /providers`

Lista os provedores, ordenados por nome do jogo, com a imagem custom por
último. Cada um traz `fields`, que é o que o wizard renderiza.

### `GET /providers/{id...}`

O ID tem barra (`itzg/minecraft-server`), daí o curinga na rota.

Um `field`:

```json
{
  "key": "MAX_PLAYERS", "label": "Máximo de jogadores", "type": "int",
  "default": "10", "min": 1, "max": 200,
  "help": "…", "required": false, "secret": false, "advanced": false
}
```

`type` é `text`, `password`, `int`, `float`, `bool` ou `enum`. Enum traz
`options: [{value, label}]`. `secret: true` mantém o valor fora do compose.

## Instâncias

### `GET /instances`

```json
{
  "instances": [ /* … */ ],
  "states": ["stopped","provisioning","starting","running","updating","error","archived"]
}
```

`states` vem na ordem das colunas do kanban, para o frontend não repetir a
lista.

Uma instância junta o que está em disco com o que o Docker respondeu agora:

```json
{
  "name": "smp-familia",
  "providerId": "itzg/minecraft-server",
  "game": "minecraft-java",
  "image": "itzg/minecraft-server:java21",
  "env": {"EULA": "true", "TYPE": "PAPER"},
  "secretKeys": ["RCON_PASSWORD"],
  "ports": [{"host": 25565, "container": 25565, "protocol": "tcp", "label": "Jogo"}],
  "mounts": [{"host": "./data", "container": "/data", "data": true}],
  "memoryLimit": "6g", "cpus": 2, "restart": "unless-stopped",
  "stopGraceSeconds": 120,
  "createdAt": "2026-08-21T12:00:00Z", "updatedAt": "2026-08-21T12:00:00Z",

  "dir": "/srv/games/smp-familia",
  "state": "running",
  "status": "Up 3 days (healthy)",
  "health": "healthy",
  "stats": {"cpuPercent": 12.4, "memoryBytes": 2040109465, "memoryLimit": 6442450944}
}
```

`env` inclui os segredos — a API é local e o formulário precisa deles para
editar. Eles só não vão para o `docker-compose.yml`.

`operation` aparece enquanto há algo em andamento:

```json
{"kind": "provision", "message": "Pulling from itzg/minecraft-server", "startedAt": "…"}
```

### `POST /instances`

```json
{
  "name": "smp-familia",
  "providerId": "itzg/minecraft-server",
  "values": {"EULA": "true", "TYPE": "PAPER", "MAX_PLAYERS": "20"},
  "memoryLimit": "6g",
  "ports": [{"host": 25565, "container": 25565, "protocol": "tcp"}],
  "start": true
}
```

Tudo além de `name`, `providerId` e `values` é opcional: portas, volumes, RAM e
CPUs caem no default do provedor. `values` passa pelo schema — campo
desconhecido é 422, exceto no provedor `custom`.

`start: true` sobe logo depois de criar; a resposta é `201` na hora e o `pull`
segue em background. Sem `start`, a instância nasce parada e o orçamento de RAM
não é conferido ainda.

### `PUT /instances/{name}`

Mesmo corpo. Se a instância estiver de pé e a mudança exigir recriar o
container, o painel derruba e sobe de novo em background (`state: updating`).
Os volumes bind ficam, então o mundo é preservado.

Mudança de variável de ambiente sempre exige recriar: env só entra no processo
quando ele nasce.

### `POST /instances/preview-compose`

Mesmo corpo, não escreve nada:

```json
{"compose": "name: smp-familia\nservices:\n  …", "recreate": ["MAX_PLAYERS", "limite de RAM"]}
```

`recreate` só vem quando já existe instância com aquele nome.

### `DELETE /instances/{name}?keepData=false`

`keepData` é `true` por padrão: apaga os arquivos gerados e preserva os
diretórios de mundo. Apagar dados de jogo é irreversível e precisa ser pedido
de propósito. `204`.

### Ações

`202 Accepted` — rodam em background, e o estado real chega pelo SSE.

- `POST /instances/{name}/start` — confere o orçamento de RAM antes
- `POST /instances/{name}/stop`
- `POST /instances/{name}/restart`
- `POST /instances/{name}/update-image` — procura imagem nova e recria **só se
  houver**. Comparar o digest local antes e depois do `pull` é o que evita
  derrubar um servidor cheio de jogadores para nada; a saída de texto do `pull`
  muda entre versões do docker e não serve para isso. O resultado chega por
  evento (`instance.updated` ou `instance.uptodate`), porque "nada a fazer" não
  muda a listagem e sem aviso a ação pareceria não ter acontecido.

`204 No Content`:

- `POST /instances/{name}/archive` — derruba e mantém os volumes
- `POST /instances/{name}/unarchive`
- `POST /instances/{name}/clear-error` — esquece uma operação que falhou

### `GET /instances/{name}/compose`

O YAML como está no disco (`text/yaml`), que pode divergir do gerado se alguém
editou à mão.

## Streams

Ambos são `text/event-stream`.

### `GET /events`

```
event: instance.changed
data: {"type":"instance.changed","instance":"smp-familia"}
```

Tipos: `instance.created`, `instance.changed`, `instance.deleted`,
`instance.failed`, `instance.progress`, `instance.updated`,
`instance.uptodate`. O evento avisa que algo mudou; o dado vem de
`GET /instances`. Os dois últimos trazem `message` pronta para a tela, porque
descrevem o desfecho de uma operação e não um estado consultável. Comentário `: ping` a cada 25 s segura a conexão.

### `GET /instances/{name}/logs?tail=300&follow=true`

```
event: log
data: "[11:04:07] joana entrou no jogo"
```

Cada linha vem como string JSON — log com quebra de linha quebraria o
enquadramento do SSE. `event: end` fecha o stream.
