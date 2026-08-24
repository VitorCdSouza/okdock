# API contract

Base: `/api/v1`. Everything is JSON except where noted. No authentication, see
the last section of `docs/architecture.md`.

## Errors

Every error returns the same body:

```json
{
  "error": "memory_budget",
  "message": "smp asks for 8g, but only 3 GB is free in the 13 GB budget (running instances already use 10 GB)",
  "params": {
    "instance": "smp",
    "requested": "8g",
    "free": "3 GB",
    "budget": "13 GB",
    "committed": "10 GB"
  }
}
```

`error` is the stable code and `params` carries what the sentence needs; those
two are what the panel uses to write the message in the language of whoever is
looking. `message` is the same sentence in English, and works as a last resort:
for whoever reads the raw JSON, and for a client that does not know the code
yet.

`problems` only comes in the 422 of `invalid_fields`, and each item points at
the field:

```json
{
  "error": "invalid_fields",
  "message": "MAX_PLAYERS: above_max",
  "problems": [
    {"field": "MAX_PLAYERS", "code": "above_max", "params": {"max": 200}}
  ]
}
```

Problem codes: `required`, `unknown_field`, `not_int`, `not_number`, `not_bool`,
`below_min`, `above_max`, `not_option`, `image_owned_by` and
`image_not_accepted`.

Some errors refine the reason inside `params.reason`: `invalid_root` uses
`not_absolute`, `create_failed`, `unreadable`, `not_dir` or `unwritable`.

| `error` | Status | When |
|---|---|---|
| `not_found` | 404 | no such instance or template |
| `already_exists` | 409 | an instance with that name already exists |
| `invalid_fields` | 422 | values outside the template schema |
| `memory_budget` | 409 | does not fit in the host RAM |
| `port_taken` | 409 | port already used by another instance |
| `docker_failed` | 409 | `docker compose` failed, `params.detail` carries the stderr |
| `external_instance` | 409 | the action does not apply to a container the panel does not own, `params.name` says which |
| `bad_request` | 400 | malformed body or unknown field |
| `dns_rejected` | 422 | duckdns answered KO: wrong token or a domain outside the account |
| `dns_unreachable` | 409 | duckdns did not answer, the server may have no way out to the internet |
| `dns_token_missing` | 409 | no token saved yet |
| `dns_taken` | 409 | another instance already uses that domain |
| `dns_disabled` | 409 | the panel started with no DNS client |
| `invalid_domain` | 422 | the name is not a valid subdomain |
| `invalid_root` | 422 | the requested root does not work, the reason comes in `params.reason` |
| `internal` | 500 | any unforeseen error |

An unknown field in the body is rejected on purpose: it is usually a typo on the
client side, and accepting it silently makes the configuration come out
different from what the user asked for.

## System

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

`root` is the root of the instance directories: it starts at `OKDOCK_ROOT` and
can be changed through `PUT /system/root`. `memoryBudget` is
`memoryTotal - memoryReserve`. `memoryCommitted` adds up the cap of the running
instances; `memoryPlanned` adds the stopped ones too. With no daemon,
`dockerError` comes instead of `dockerVersion`.

### `PUT /system/root`

```json
{"root": "/mnt/disk/games"}
```

New instances start being written there, and the updated `GET /system` comes
back. The path has to be absolute and writable; the panel creates the directory
if it is missing and answers `422 invalid_root` when it cannot.

Instances that already exist **do not move**: docker keeps the absolute path of
the bind mounts, so they stay up where they are and disappear from the listing
until the root comes back. The choice is saved in
`<OKDOCK_ROOT>/.okdock/config.json`, on the boot root and not on the new one,
otherwise the next process would look for the file in the wrong place.

### `GET /health`

`{"status":"ok"}`. Does not touch Docker, it is the container healthcheck.

## Templates

A template describes an image: category, ports, volumes, RAM and the fields the
form shows. The ones shipping with OkDock are JSON embedded in the binary; the
user ones live in `<boot root>/.okdock/templates/<id>.json`, and a file with the
same id beats the builtin one, which is how a builtin template is edited without
losing the original.

### `GET /templates`

```json
{
  "templates": [ /* … */ ],
  "categories": ["games","media","database","network","utilities","other"]
}
```

Sorted by category and name, with the loose image last. `categories` comes along
so the frontend does not repeat the list: the six shipped ones, then the ones
some template invented, in alphabetical order, and `other` closing the list. `builtin: true` marks what shipped
with OkDock and has not been edited yet.

### `GET /templates/{id}`

The id is the file name on disk: lowercase, digits and hyphen, 2 to 40
characters. An instance created when the field was called `providerId` saved an
id with a slash (`itzg/minecraft-server`); the lookup translates those three old
ids into the new ones.

### `POST /templates`

Creates. `409 template_exists` if the id already exists, builtin ones included.

### `PUT /templates/{id}`

Writes over, including over a builtin template: the edit becomes a file on disk
and is what the API serves from then on.

### `DELETE /templates/{id}`

Deletes the file on disk. If the id is also a builtin one, the template **goes
back to the original** instead of disappearing; if there never was a file,
`409 template_builtin`.

A rejected template answers `422 invalid_fields` with the same `problems` list
as instance creation: `bad_template_id`, `unknown_category`, `bad_memory`,
`bad_port`, `duplicate_port`, `container_path_not_absolute`,
`many_data_volumes`, `duplicate_field`, `bad_field_type`,
`enum_without_options`, `arg_without_flag`.

A `field`:

```json
{
  "key": "MAX_PLAYERS", "label": "Max players", "type": "int",
  "default": "10", "min": 1, "max": 200,
  "help": "…", "required": false, "secret": false, "advanced": false
}
```

`type` is `text`, `password`, `int`, `float`, `bool` or `enum`. An enum carries
`options: [{value, label}]`. `secret: true` keeps the value out of the compose.

`label` and `help` come in English. Whoever builds the screen looks first for
the template + key pair (`field.minecraft-java.MEMORY.help`) in the table of the
chosen language, and only uses the text from here when it finds nothing. That
way a new template enters the catalog without depending on the frontend, showing
up in English until it gets a translation. The port `label` is a code (`game`)
for the same reason.

## Images

### `GET /images?q=jellyfin&limit=25`

```json
{
  "images": [
    {"name": "jellyfin/jellyfin", "description": "Jellyfin media server", "stars": 1200},
    {"name": "linuxserver/jellyfin", "description": "…", "stars": 800, "official": true}
  ]
}
```

This is `docker search`, so the daemon is the one reaching the registry and the
panel needs no outbound route of its own. Two limits come with that: it only
searches **Docker Hub**, so nothing from `ghcr.io` or `lscr.io` shows up, and it
answers with repositories, never with tags. An empty `q` answers an empty list
without asking docker anything. `limit` defaults to 25 and is capped at 100.

### `GET /images/tags?image=jellyfin/jellyfin`

```json
{"tags": ["10.9.11", "10.9.10", "latest"]}
```

Newest first, except `latest`, which always leads when it exists: an image
that pushes a nightly every day buries it past the newest page, so it is asked
for by name when the page does not carry it. This one does not go through the
daemon: it is the Docker Hub
API, called by the panel itself, which is its only outbound request besides
duckdns. An image hosted anywhere else answers `409 tags_not_hub`, a Hub that
did not answer is `409 registry_unreachable`, and a repository that does not
exist is an empty list, not an error. A tag in `image` is ignored, only the
repository part is used.

## Instances

### `GET /instances`

```json
{
  "instances": [ /* … */ ],
  "states": ["stopped","provisioning","starting","running","updating","archived","error"]
}
```

`states` comes in the order the panel presents the states, so the frontend does
not repeat the list. Not every state becomes a column: `provisioning` and
`starting` are transitions, and the board shows those cards in the `running`
column already.

An instance joins what is on disk with what Docker answered just now:

```json
{
  "name": "smp-family",
  "templateId": "minecraft-java",
  "category": "games",
  "image": "itzg/minecraft-server:java21",
  "env": {"EULA": "true", "TYPE": "PAPER"},
  "secretKeys": ["RCON_PASSWORD"],
  "ports": [{"host": 25565, "container": 25565, "protocol": "tcp", "label": "game"}],
  "mounts": [{"host": "./data", "container": "/data", "data": true}],
  "memoryLimit": "6g", "cpus": 2, "restart": "unless-stopped",
  "stopGraceSeconds": 120,
  "createdAt": "2026-08-21T12:00:00Z", "updatedAt": "2026-08-21T12:00:00Z",

  "dir": "/srv/games/smp-family",
  "networks": ["smp-family_default"],
  "state": "running",
  "status": "Up 3 days (healthy)",
  "health": "healthy",
  "stats": {"cpuPercent": 12.4, "memoryBytes": 2040109465, "memoryLimit": 6442450944}
}
```

A Spec saved when a template was called a provider has `providerId` and `game`
in place of the first two; the read accepts both names and the next write swaps
them for the new ones.

`networks` is what `docker ps` answered for that container. It is absent while
Docker does not answer. What the board groups by is `project`, the compose
project the container belongs to, which the panel reads from the
`com.docker.compose.project` label.

`env` includes the secrets: the API is local and the form needs them to edit.
They are only kept out of `docker-compose.yml`.

`dns` shows up when the instance has a dynamic DNS name linked:

```json
{"domain": "smp", "hostname": "smp.duckdns.org", "lastIp": "187.12.3.4", "lastSync": "…"}
```

It comes along with the instance because the name is only useful glued to the
port: what the card shows for copying is `smp.duckdns.org:25565`.

`operation` shows up while something is in flight:

```json
{"kind": "provision", "message": "Pulling from itzg/minecraft-server", "startedAt": "…"}
```

`code` is the step when the panel is the one speaking (`preparing`, `creating`,
`starting`, `restarting`, `stopping`, `recreating`, `checking_update`,
`recreating_new_image`, `starting_new_config`), and then `message` comes empty.
Only the raw `docker` line travels in `message`, because there is nothing to
translate in it.

### `POST /instances`

```json
{
  "name": "smp-family",
  "templateId": "minecraft-java",
  "values": {"EULA": "true", "TYPE": "PAPER", "MAX_PLAYERS": "20"},
  "memoryLimit": "6g",
  "ports": [{"host": 25565, "container": 25565, "protocol": "tcp"}],
  "start": true
}
```

Everything beyond `name`, `templateId` and `values` is optional: ports, volumes,
RAM and CPUs fall back to the template default. `values` goes through the
schema, and an unknown field is a 422, except in a template with `freeEnv`, like
`custom`.

`start: true` brings it up right after creating; the answer is `201` right away
and the `pull` goes on in the background. Without `start`, the instance is born
stopped and the RAM budget is not checked yet.

### `PUT /instances/{name}`

Same body. If the instance is up and the change requires recreating the
container, the panel takes it down and brings it up again in the background
(`state: updating`). The bind volumes stay, so the world is preserved.

An environment variable change always requires a recreate: env only enters the
process when it is born.

### `POST /instances/preview-compose`

Same body, writes nothing:

```json
{"compose": "name: smp-family\nservices:\n  …", "recreate": ["MAX_PLAYERS", "RAM limit"]}
```

`recreate` only comes when an instance with that name already exists.

### `DELETE /instances/{name}?keepData=false`

`keepData` is `true` by default: it deletes the generated files and keeps the
world directories. Deleting game data is irreversible and has to be asked for on
purpose. `204`.

### Actions

`202 Accepted`, they run in the background and the real state arrives over SSE.

- `POST /instances/{name}/start`, checks the RAM budget first
- `POST /instances/{name}/stop`
- `POST /instances/{name}/restart`
- `POST /instances/{name}/update-image`, looks for a new image and recreates
  **only if there is one**. Comparing the local digest before and after the
  `pull` is what avoids taking a server full of players down for nothing; the
  text output of `pull` changes between docker versions and is no good for that.
  The result arrives as an event (`instance.updated` or `instance.uptodate`),
  because "nothing to do" does not change the listing and without a notice the
  action would look like it never happened.

`204 No Content`:

- `POST /instances/{name}/archive`, takes it down and keeps the volumes
- `POST /instances/{name}/unarchive`
- `POST /instances/{name}/clear-error`, forgets a failed operation

On an external container (`external: true` in the listing) only `start`, `stop`,
`restart` and `clear-error` apply: the panel calls `docker start/stop/restart`
by container name, in the background as well, and a docker failure becomes an
operation with an error, visible on the card. The other actions answer
`409 external_instance`, a refusal on purpose and not a failure: editing,
updating and deleting belong to the original compose.

### `GET /instances/{name}/compose`

The YAML as it is on disk (`text/yaml`), which may differ from the generated one
if someone edited it by hand.

## Dynamic DNS

A fixed name for a residential IP that changes, through
[duckdns.org](https://www.duckdns.org). Two limitations of the service run
through this whole contract:

- **The duckdns API does not create subdomains.** The name is born on the site.
  Here it is only possible to update the IP of one that exists, and that is what
  doubles as verification: `OK` means that token controls that name.
- **There is only name to IP.** There is no SRV, so there is no port in DNS.
  Whoever joins needs `name:port`, and the port still depends on the router,
  which the panel has no way to check.

### `GET /dns`

```json
{
  "token": "a1b2c3d4-…",
  "suffix": ".duckdns.org",
  "links": [
    {"instance": "smp-family", "domain": "smp", "hostname": "smp.duckdns.org",
     "lastIp": "187.12.3.4", "lastSync": "2026-08-21T12:00:00Z"}
  ],
  "domains": [
    {"domain": "smp", "hostname": "smp.duckdns.org",
     "lastIp": "187.12.3.4", "lastSync": "2026-08-21T12:00:00Z"}
  ]
}
```

`links` is the name of each instance; `domains` is the list of names in the
account, registered on the settings screen, with or without an instance using
each one. Linking a name to an instance also registers it.

The token comes in the answer for the same reason `env` carries the secrets: the
API is local and the form needs the value to edit. On disk it lives in
`<boot root>/.okdock/dns.json`, with `0600`, and never enters any compose.
`suffix` comes from here so the frontend does not repeat the rule.

### `PUT /dns`

`{"token": "…"}`. Returns the same body as the `GET`. Saving a new token fires a
sync of the domains already linked.

### `POST /dns/domains`

`{"domain": "smp"}`, accepting `smp`, `SMP` or `smp.duckdns.org`. Registering
**is** verifying, just like linking: the call updates the IP at duckdns and only
saves if it answers `OK`. `200` with the registered name, or `422 dns_rejected`.

### `DELETE /dns/domains/{domain}`

Takes the name off the panel list. `204`. The subdomain keeps existing in the
duckdns account, since their API does not delete names and only the site can do
that, and an instance already using that name stays linked.

### `PUT /instances/{name}/dns`

`{"domain": "smp"}`, accepting `smp`, `SMP` or `smp.duckdns.org`, and keeping
the label. Linking **is** verifying: the call updates the IP at duckdns and only
saves if it answers `OK`. `200` with the link, or `422 dns_rejected`.

### `DELETE /instances/{name}/dns`

`204`. Undoes the link in the panel; the name keeps existing in the duckdns
account, which only the site deletes.

### `POST /dns/sync`

`202`. Resends the IP of every domain now, without waiting for the 5 minute
cycle. Deleting the instance also deletes the link.

## Streams

Both are `text/event-stream`.

### `GET /events`

```
event: instance.changed
data: {"type":"instance.changed","instance":"smp-family"}
```

Types: `instance.created`, `instance.changed`, `instance.deleted`,
`instance.failed`, `instance.progress`, `instance.updated`,
`instance.uptodate`, `dns.changed`. The event says something changed; the data
comes from `GET /instances`. The last two describe the outcome of an operation
rather than a state that can be queried: the panel builds the sentence from the
event type and the instance name, and the `message` coming along is the English
version. `dns.changed` only goes out when the IP (or the failure) of some domain
really changes, since publishing on every cycle would make the panel reload
everything every 5 minutes with nothing having happened. A `: ping` comment
every 25 s holds the connection.

### `GET /instances/{name}/logs?tail=300&follow=true`

```
event: log
data: "[11:04:07] joana joined the game"
```

Every line comes as a JSON string, because a log with a line break would break
the SSE framing. `event: end` closes the stream.
