# Architecture

## The API layers

```
cmd/okdock          flags, logger, HTTP server, graceful shutdown
  └── httpapi         REST + SSE. Only translates HTTP, knows no Docker.
        └── manager   orchestration: RAM budget, port conflicts,
                      background operations, state derivation
              ├── store     disk persistence (compose, .env, metadata)
              │     └── compose   YAML and .env generation
              ├── dockerx   `docker compose` behind an interface
              ├── duckdns   dynamic DNS behind an interface
              ├── template  templates and field schema
              └── system    host RAM, CPU and disk
```

There is a single rule: nothing above `manager` knows Docker, and nothing below
it knows HTTP. `httpapi` turns a domain error into a status:

| Error | Status |
|---|---|
| `store.ErrNotFound` | 404 |
| `store.ErrExists` | 409 |
| `template.ValidationError` | 422, with the field list |
| `template.ErrNotFound`, `ErrBuiltin` | 404 and 409 in the template CRUD |
| `manager.ErrBudget`, `ErrPortTaken` | 409 |
| `dockerx.Error` | 409, docker failed over something the user can fix |
| `duckdns.ErrRejected` | 422, the service answered and the user is the one who fixes it |
| `duckdns.ErrUnreachable`, `manager.ErrNoToken`, `ErrDNSTaken` | 409 |

## Decisions that are not an accident

### The compose on disk is the source of truth

The panel does not create containers through the SDK. It writes a
`docker-compose.yml` and calls the `docker compose` binary. That costs one
process per operation, and in exchange the server stays manageable from the
terminal if the panel dies, which is the likely scenario on a home server.

And it is read back, not only written. `store.Get` parses the file and lets it
answer for image, ports, volumes, environment, limits, restart and stop grace,
so editing the YAML by hand is a supported way to change an instance. When the
parse fails, the sidecar answers instead, which is how an instance written
before this existed, or one whose file the owner broke, is still read.

`.okdock.json` keeps only what the compose cannot say: which template it came
from, which keys are secret, which mount holds the data, what each port is
called, whether the instance is archived and when it was created. The password
is not there, it is only in the `.env`, because the sidecar is `0644`.

### A container from outside is edited in its own file

`docker ps` reports almost nothing: no environment, no volumes, no memory
limit. But docker also records which compose file each container came from, so
the panel reads that file and gets the whole configuration, the same way it
reads its own.

The panel runs in a container, so a path the daemon reports is not necessarily
a path it can open. When it cannot, the container stays read only and the
screen says which of the four reasons it is (`no_compose`, `not_visible`,
`unreadable`, `unsupported`) instead of silently offering less. The fix for
`not_visible` is a bind mount of that folder into the panel, at the same path
on both sides, which the compose CLI needs anyway: it reads the file inside the
panel while the daemon resolves that file bind mounts outside.

Saving writes back only that one service, through `compose.Apply`, which edits
the YAML node in place: the other services of the stack, the comments and the
anchors survive. Then only that service comes back up, with `--no-deps`. The
panel refuses to write whenever the parse found something it cannot express, an
`include` or a port range, since rewriting the file would drop it.

The form of an outside container only knows the fields of the template guessed
from the image, so what it sends is merged into the environment instead of
replacing it. Deleting stays out: removing a service from a compose file the
panel does not own is not the panel's call.

### The name is one name

Directory name, compose project `name:` and `container_name` are the same text,
and `store.Get` rewrites the name from the directory. With no explicit `name:`
the project inherits the folder name, and when the two diverge `docker compose`
acts on a project other than the one the folder suggests.

### A secret never goes into the compose

Fields marked `secret: true` in the template stay out of the YAML: they go to an
`.env` with `0600`, referenced by `env_file`. The list of secret keys is saved in
the instance Spec, it is not read back from the template, so a password already
saved does not start leaking if the schema changes later.

Non-secret fields stay inline in `environment:`, because being able to read the
configuration from the terminal is half the reason to generate a compose at all.

### The DNS link does not belong to the instance

The duckdns domain does not change a single line of `docker-compose.yml`. If it
lived in the `Spec`, it would enter the body of `PUT /instances`, the
`preview-compose` and the logic that decides whether the container has to be
born again: a field with no effect crossing exactly the machinery that exists to
answer "does this need a recreate?".

So it lives in `<boot root>/.okdock/dns.json`, next to the token, with `0600`.
On the root the process started with, not the current instance root, because it
is the neighbouring `config.json` in that same folder that says where the root
was moved to. One file, one lock, and the instance directory stays being only
what docker reads. The price is clearing the link when the instance is deleted,
and skipping on read whatever points at an instance that no longer exists.

Linking and checking are the same call because the duckdns API does not create
subdomains: it only updates a name that already exists, and an `OK` from it is
the proof that the token controls that name.

And what reaches the screen is always `name:port`, never the name alone. Duckdns
maps name to IP only, there is no SRV, so half the road is still the port
forwarding on the router, which the panel cannot check from inside the LAN and
therefore warns about instead of promising.

### The RAM budget is checked first

`manager.checkBudget` adds up the memory cap of the **running** instances and
compares it against `total RAM - reserve` before any `up`. Without that the
limit only shows up as `Exited (137)`, after the kernel kills the process.

A stopped instance does not count: it takes nothing.

### Long operations run loose

`pull` and `up` take minutes. They run in a goroutine with
`context.Background()`, not with the request context, because closing the
browser tab must not abort a 2 GB download. Progress lives in an in-memory
operation registry, and that is what fills the progress bar on the card.

Accepted consequence: restarting the panel forgets the operations in flight. The
real state comes back on the next `docker compose ps`.

The DNS sync follows the same rule, on a 5 minute ticker with the process
context: keeping the name pointing here cannot depend on a tab being open. With
no token configured it never goes to the network.

### SSE announces, it does not carry data

The event stream sends `{type, instance}` and nothing else. The frontend reloads
the list. That keeps the API as the single source of truth, since reconciling a
partial mutation on the client is how a ghost card appears on the kanban. The
Angular `Store` groups events in a 250 ms window, because a `docker pull` emits
dozens of lines per second.

### The Docker runner is an interface

The development machine has no daemon. `dockerx.Runner` has two implementations:
`CLI`, which calls the binary, and `Fake`, in memory. Every `manager` test runs
on the `Fake`, so `go test ./...` passes without Docker.

`dockerx.Error` carries the stderr of the command. Without it the error would be
`exit status 1`, while the real reason (`port is already allocated`, `no space
left on device`) sits in stderr.

## The frontend

Angular 20 standalone, with signals. No NgRx: the state fits in one service
(`core/state.ts`) with four signals and a few `computed`.

`shared/template-form` builds the form from the schema the API sent. The
frontend knows no variable of any image: a new template lands without touching
code, and a builtin template is a `.json` in `internal/template/builtin/`.

No interface text is literal in a template: everything goes through `core/i18n`,
which resolves the key against the table of the current language. The Portuguese
table is what defines the `MessageKey` type, and the English one is a
`Record<MessageKey, string>`, so a new key with no translation does not compile.
The chosen language lives in `localStorage`, next to the other preferences of
whoever is looking: with no login, saving that on the server would change the
screen for everyone in the house.

The API does not send a finished sentence either: an error becomes `error` +
`params`, a rejected field becomes `{field, code, params}`, and the step of an
operation becomes `code`. The one writing the sentence is always the panel, with
the table of the current language. The text that still travels in `message` is
the last resort: a code the screen does not know falls back to it instead of
showing up empty. Template text (field `help`, description) follows the same
rule: the screen looks for the key `field.<template>.<FIELD>.help` and falls back
to the API text when it does not find one, which is the only possible road for a
template registered through the panel. Log lines and container status belong to
docker and pass through untouched.

In production the Go binary serves the Angular `dist/` (through `go:embed`) and
the API on the same origin: one container, one port, no CORS. In development
`ng serve` proxies `/api` to `localhost:8080`.

## What does not exist yet

- **Authentication.** The panel controls `docker.sock`; whoever reaches port
  8080 controls the server. Today that depends on it not being exposed outside
  the LAN. Before any exposure, it needs a login.

  Dynamic DNS does not change that by itself, since a name opens no port and
  only the forwarded ones answer, but from then on there is a public name
  pointing at the house. Forwarding 8080 along with the game port would hand
  `docker.sock` to the whole internet.
- **World upload and file browser**: the tab exists in the design.
- **Backups**: the tab exists in the design.
- **Interactive console**: today the log is read only, sending a command (RCON
  or `docker attach`) is missing.
- **Rebuilding the operation state** after a panel restart.
