# Contrato da API

Base: `/api/v1`. Tudo JSON, exceto onde indicado. Sem autenticação — ver a
seção final de `docs/arquitetura.md`.

## Erros

Todo erro devolve o mesmo corpo:

```json
{
  "error": "memory_budget",
  "message": "smp pede 8g, mas só há 3 GB livres no orçamento de 13 GB (as instâncias de pé já usam 10 GB)",
  "params": {
    "instance": "smp",
    "requested": "8g",
    "free": "3 GB",
    "budget": "13 GB",
    "committed": "10 GB"
  }
}
```

`error` é o código estável e `params` traz o que a frase precisa — é com esses
dois que o painel escreve a mensagem no idioma de quem está olhando. `message`
é a mesma frase em português, e serve de último recurso: para quem lê o JSON
cru e para um cliente que ainda não conhece o código.

`problems` só vem no 422 de `invalid_fields`, e cada item aponta o campo:

```json
{
  "error": "invalid_fields",
  "message": "MAX_PLAYERS: above_max",
  "problems": [
    {"field": "MAX_PLAYERS", "code": "above_max", "params": {"max": 200}}
  ]
}
```

Códigos de problema: `required`, `unknown_field`, `not_int`, `not_number`,
`not_bool`, `below_min`, `above_max`, `not_option`, `image_owned_by` e
`image_not_accepted`.

Alguns erros afinam o motivo dentro de `params.reason` — `invalid_root` usa
`not_absolute`, `create_failed`, `unreadable`, `not_dir` ou `unwritable`.

| `error` | Status | Quando |
|---|---|---|
| `not_found` | 404 | instância ou provedor inexistente |
| `already_exists` | 409 | já existe instância com esse nome |
| `invalid_fields` | 422 | valores fora do schema do provedor |
| `memory_budget` | 409 | não cabe na RAM do host |
| `port_taken` | 409 | porta já usada por outra instância |
| `docker_failed` | 409 | o `docker compose` falhou; `params.detail` traz o stderr |
| `bad_request` | 400 | corpo malformado ou campo desconhecido |
| `dns_rejected` | 422 | o duckdns respondeu KO: token errado ou domínio que não é da conta |
| `dns_unreachable` | 409 | o duckdns não respondeu; o servidor pode estar sem saída para a internet |
| `dns_token_missing` | 409 | ainda não há token gravado |
| `dns_taken` | 409 | outra instância já usa esse domínio |
| `dns_disabled` | 409 | o painel subiu sem cliente de DNS |
| `invalid_domain` | 422 | o nome não é um subdomínio válido |
| `invalid_root` | 422 | a raiz pedida não serve; o motivo vem em `params.reason` |
| `internal` | 500 | qualquer erro não previsto |

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
  "root": "/srv/games",
  "dockerVersion": "27.1.0",
  "memoryReserve": 2147483648,
  "memoryBudget": 13958643712,
  "memoryCommitted": 6442450944,
  "memoryPlanned": 12884901888,
  "instanceCount": 4
}
```

`root` é a raiz dos diretórios de instância — começa em `OKDOCK_ROOT` e pode
ser trocada por `PUT /system/root`. `memoryBudget` é
`memoryTotal − memoryReserve`. `memoryCommitted` soma o teto
das instâncias de pé; `memoryPlanned` soma também as paradas. Sem daemon, vem
`dockerError` no lugar de `dockerVersion`.

### `PUT /system/root`

```json
{"root": "/mnt/disco/jogos"}
```

Passa a gravar as instâncias novas ali e devolve o `GET /system` já atualizado.
O caminho tem que ser absoluto e gravável; o painel cria o diretório se faltar
e responde `422 invalid_root` quando não dá.

As instâncias que já existem **não se movem**: o docker guarda o caminho
absoluto dos bind mounts, então elas continuam de pé onde estão e somem da
listagem até a raiz voltar. A escolha fica gravada em
`<OKDOCK_ROOT>/.okdock/config.json` — na raiz de boot, e não na nova, senão
o processo seguinte procuraria o arquivo no lugar errado.

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

`label` e `help` vêm em português. Quem monta a tela procura primeiro pelo par
provedor + chave (`field.itzg/minecraft-server.MEMORY.help`) na tabela do
idioma escolhido, e só usa o texto daqui quando não acha — assim um jogo novo
entra no catálogo sem depender do frontend, aparecendo em português até ganhar
tradução. O `label` da porta é um código (`game`) pela mesma razão.

## Instâncias

### `GET /instances`

```json
{
  "instances": [ /* … */ ],
  "states": ["stopped","provisioning","starting","running","updating","archived","error"]
}
```

`states` vem na ordem em que o painel apresenta os estados, para o frontend não
repetir a lista. Nem todo estado vira coluna: `provisioning` e `starting` são
passagem, e o quadro mostra esses cards já na coluna `running`.

Uma instância junta o que está em disco com o que o Docker respondeu agora:

```json
{
  "name": "smp-familia",
  "providerId": "itzg/minecraft-server",
  "game": "minecraft-java",
  "image": "itzg/minecraft-server:java21",
  "env": {"EULA": "true", "TYPE": "PAPER"},
  "secretKeys": ["RCON_PASSWORD"],
  "ports": [{"host": 25565, "container": 25565, "protocol": "tcp", "label": "game"}],
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

`dns` aparece quando a instância tem nome de DNS dinâmico vinculado:

```json
{"domain": "smp", "hostname": "smp.duckdns.org", "lastIp": "187.12.3.4", "lastSync": "…"}
```

Ele vem junto da instância porque o nome só serve colado na porta: o que o card
mostra para copiar é `smp.duckdns.org:25565`.

`operation` aparece enquanto há algo em andamento:

```json
{"kind": "provision", "message": "Pulling from itzg/minecraft-server", "startedAt": "…"}
```

`code` é a etapa quando quem fala é o painel — `preparing`, `creating`,
`starting`, `restarting`, `stopping`, `recreating`, `checking_update`,
`recreating_new_image`, `starting_new_config` — e aí `message` vem vazia. Só a
linha crua do `docker` viaja em `message`, porque não há o que traduzir nela.

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

## DNS dinâmico

Um nome fixo para um IP residencial que muda, via [duckdns.org](https://www.duckdns.org).
Duas limitações do serviço atravessam todo este contrato:

- **A API do duckdns não cria subdomínio.** O nome nasce no site. Aqui só dá
  para atualizar o IP de um que já existe — e é isso que serve de verificação:
  `OK` quer dizer que aquele token controla aquele nome.
- **Só existe nome → IP.** Não há SRV, então não há porta no DNS. Quem for
  entrar precisa de `nome:porta`, e a porta ainda depende do roteador — o
  painel não tem como conferir essa parte.

### `GET /dns`

```json
{
  "token": "a1b2c3d4-…",
  "suffix": ".duckdns.org",
  "links": [
    {"instance": "smp-familia", "domain": "smp", "hostname": "smp.duckdns.org",
     "lastIp": "187.12.3.4", "lastSync": "2026-08-21T12:00:00Z"}
  ],
  "domains": [
    {"domain": "smp", "hostname": "smp.duckdns.org",
     "lastIp": "187.12.3.4", "lastSync": "2026-08-21T12:00:00Z"}
  ]
}
```

`links` é o nome de cada instância; `domains` é a lista de nomes da conta,
cadastrada na tela de configurações, com ou sem instância usando cada um.
Vincular um nome a uma instância também o cadastra.

O token vem na resposta pelo mesmo motivo que `env` traz os segredos: a API é
local e o formulário precisa do valor para editar. Em disco ele fica em
`<raiz de boot>/.okdock/dns.json`, com `0600`, e nunca entra em compose
nenhum. `suffix` vem daqui para o frontend não repetir a regra.

### `PUT /dns`

`{"token": "…"}`. Devolve o mesmo corpo do `GET`. Gravar um token novo dispara
um sync dos domínios já vinculados.

### `POST /dns/domains`

`{"domain": "smp"}` — aceita `smp`, `SMP` ou `smp.duckdns.org`. Cadastrar **é**
verificar, igual ao vínculo: a chamada atualiza o IP no duckdns e só grava se
ele responder `OK`. `200` com o nome cadastrado, ou `422 dns_rejected`.

### `DELETE /dns/domains/{domain}`

Tira o nome da lista do painel. `204`. O subdomínio continua existindo na conta
do duckdns — a API deles não apaga nome, isso só dá para fazer no site —, e uma
instância que já usa esse nome segue vinculada.

### `PUT /instances/{name}/dns`

`{"domain": "smp"}` — aceita `smp`, `SMP` ou `smp.duckdns.org`, e guarda o
rótulo. Vincular **é** verificar: a chamada atualiza o IP no duckdns e só grava
se ele responder `OK`. `200` com o vínculo, ou `422 dns_rejected`.

### `DELETE /instances/{name}/dns`

`204`. Desfaz o vínculo no painel; o nome continua existindo na conta do
duckdns, que só o site apaga.

### `POST /dns/sync`

`202`. Reenvia o IP de todos os domínios agora, sem esperar o ciclo de 5
minutos. Excluir a instância também apaga o vínculo.

## Streams

Ambos são `text/event-stream`.

### `GET /events`

```
event: instance.changed
data: {"type":"instance.changed","instance":"smp-familia"}
```

Tipos: `instance.created`, `instance.changed`, `instance.deleted`,
`instance.failed`, `instance.progress`, `instance.updated`,
`instance.uptodate`, `dns.changed`. O evento avisa que algo mudou; o dado vem de
`GET /instances`. Os dois últimos descrevem o desfecho de uma operação, e não
um estado consultável: o painel monta a frase a partir do tipo do evento e do
nome da instância; a `message` que vem junto é a versão em português.
`dns.changed` só sai quando o IP (ou a falha) de algum domínio muda de verdade
— publicar a cada volta do ciclo faria o painel recarregar tudo de 5 em 5
minutos sem nada ter acontecido. Comentário `: ping` a cada 25 s segura a conexão.

### `GET /instances/{name}/logs?tail=300&follow=true`

```
event: log
data: "[11:04:07] joana entrou no jogo"
```

Cada linha vem como string JSON — log com quebra de linha quebraria o
enquadramento do SSE. `event: end` fecha o stream.
