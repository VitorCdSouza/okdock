# OkDock

Panel to create and run game servers in Docker on a home server. Every instance
becomes a directory with its own `docker-compose.yml`, and the panel runs
`docker compose` on top of it. Nothing is created through the Docker SDK, and
that is on purpose: if the panel goes down, everything stays manageable from the
terminal.

```
okdock/
├── api/     Go API: templates, compose generation, orchestration
├── web/     Angular 20 panel
├── docs/    architecture, API contract, templates
└── Dockerfile, docker-compose.yml, Makefile
```

## Why a monorepo with a single branch

`api/` and `web/` share a contract. One branch per app would rule out the commit
that changes the API answer and the client consuming it at the same time: it
would always be two commits that never meet, and a merge between them would make
no sense. With folders on the same branch the change is atomic, and the CI path
filters make sure touching Angular does not run the Go CI.

## Running in development

```bash
make dev
```

Brings the API up on `:8080` and Angular on `:4200` in a single terminal, with
the output of both prefixed by `[api]` and `[web]`. Ctrl-C takes both down. Open
**http://localhost:4200**, using `localhost` and not `127.0.0.1`, because
`ng serve` listens on IPv6 only.

If you prefer one terminal each, `make dev-api` and `make dev-web` are still
there.

In this mode the API writes instances, config and templates under `./.data`, so
you can poke around without touching the real folder.

None of this needs Docker installed on the development machine: with no daemon
the panel opens, lists nothing and says docker did not answer. The tests pass
without a daemon too, because the runner is an interface with a `Fake`.

```bash
make test    # go test ./... + the Angular tests
make lint    # go vet + gofmt
```

## Running on the server

```bash
docker compose up -d
```

Every push to `main` builds the image on GitHub Actions, runs the smoke test on
it and publishes `ghcr.io/vitorcdsouza/okdock:latest` (also tagged with the
commit sha). The service says `pull_policy: always`, so the line above pulls
whatever is current and recreates the container when it changed. Nothing is
built on the server.

`docker compose restart` is not enough: it restarts the container with the image
it already has. What updates is `up -d`.

To build on your own machine instead of taking the published image, the
`build: .` is still there: `docker compose up -d --build`.

`make deploy` is the short way in. It builds the image here, hands it to the
server over ssh and recreates the container with `--pull never`, so what runs is
what was just built. The whole thing takes seconds, while going through the CI
waits for GitHub to schedule a runner, which is minutes and out of anyone
control. The workflows keep running on every push, as the check they always
were, and `ghcr.io` keeps the tag per commit to fall back to.

The panel comes up on `:8090`, mapped to the `:8080` it listens on inside the
container, with the frontend embedded in the binary itself: one container, no
separate web server. The outside port is 8090 because 8080 on the server belongs
to nextcloud.

Three things in the deploy `docker-compose.yml` are not details:

- **`/var/run/docker.sock` mounted.** It is the only road to Docker; the panel
  speaks neither TCP nor SSH.
- **`${PWD}/..:${PWD}/..`, the same path on both sides.** The folder that holds
  the panel folder, which is where the instances go, and the only folder the
  settings screen offers. `${PWD}` is filled in by compose itself, so the file
  carries nobody path, and the daemon collapses the `..` on both sides. The bind
  mounts of the generated compose are resolved by the *host* daemon, which
  cannot see the container filesystem, so a path that reads differently on the
  two sides creates the world in the wrong place, silently. Mount more lines to
  reach somewhere else, always with the same text twice.
- **`OKDOCK_CONFIG` and `OKDOCK_TEMPLATES` under `${PWD}`.** The panel
  configuration and the templates written in it, in `config/` and `templates/`
  next to the compose file of the panel. They need no mount of their own: the
  folder above is already mounted, and reading them at their real path is what
  keeps one folder from having two names inside the container. Nothing of the
  panel is written inside the instance folder anymore.

The instance folder is not in the file: it is chosen on the settings screen, out
of what is mounted above, and the panel starts with none. Until one is picked,
the board still shows the containers that Docker reports and says what is
missing.

The container carries its own `docker compose` (the Alpine plugin), so it does
not depend on the version installed on the host.

### Variables

| Variable | Default | What for |
|---|---|---|
| `OKDOCK_ADDR` | `:8080` | listen address |
| `OKDOCK_CONFIG` | `/config` | folder for `.okdock/`, the panel own files; `${PWD}/config` in the deploy |
| `OKDOCK_TEMPLATES` | `/templates` | templates written in the panel, until another folder is chosen; `${PWD}/templates` in the deploy |
| `OKDOCK_ROOT` | empty | instance folder to start with, when none was chosen yet |
| `OKDOCK_MEMORY_RESERVE` | `2g` | RAM kept outside the instance budget |
| `OKDOCK_ALLOW_ORIGIN` | empty | opens CORS, only for `ng serve` |
| `OKDOCK_LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |

## What the panel does

- **Kanban by state**: stopped, provisioning, starting, running, updating,
  error, archived. It refreshes itself over SSE. The full column gets wider and
  breaks the cards into strips instead of becoming an endless vertical roll next
  to empty columns; if the board still runs past the screen, a tag on the edge
  says which column was left out.
- **Templates by category**: games, media, database, network, utilities. The
  Templates screen shows one category per tab, and **+ Category** opens a
  category of your own, which starts existing with the first template saved in
  it. Four templates ship with the panel; the rest you register through the
  **New template** button, giving image, ports, volumes and the configuration
  fields. Editing a builtin template saves a copy of your own, and deleting the
  copy brings the original back.
- **New instance wizard**, on the `＋` of the STOPPED column. The form fields
  come from the chosen template, they are not fixed in the frontend. The image
  repository is the one from the template and only the tag is picked, `latest`
  included. The generated `docker-compose.yml` sits in the **compose.yml** tab
  of the instance. One of the DNS names already registered in the settings can
  be linked right there, without going through the instance screen.
- **RAM budget**: the panel adds up the memory cap of the running instances
  before letting one more come up. That is what keeps you from discovering the
  limit through `Exited (137)`.
- **A fixed name to invite people to**: links a
  [duckdns.org](https://www.duckdns.org) subdomain to the instance and keeps the
  IP current on its own, so the card can show `smp.duckdns.org:25565` ready to
  copy. The subdomain has to exist in the account first: the duckdns API does
  not create names, it only updates IPs. And it knows nothing about ports, so
  forwarding the port on the router is still manual work.
- **A stack in one tile**: containers of the same compose project collapse into
  a single tile on the board, the way apps sit inside a folder. Clicking it
  opens the group in place: nextcloud, its database and its cache come out
  together, and the board stays readable while they are closed.
- **Live console** and a read of `docker-compose.yml` as it is on disk.
- **Whatever was already running on the server**: a container created outside
  the panel shows up on the board all the same, with the category guessed from
  the image name, the labels and the ports. It can be stopped, started,
  restarted and its console read; the rest still belongs to the original
  compose, and the panel says so on screen instead of leaving the button looking
  dead.
- **Settings** (the gear): instance root, Docker version, duckdns token with the
  list of names in the account (each with the IP the service confirmed), which
  numbers show up in the top bar, and the interface language (Portuguese,
  English, or whatever the browser asks for).

The language choice and the choice of which numbers show in the bar live in the
browser `localStorage`, not on the server: with no login, a preference saved
there would apply to everyone in the house. The API never sends a finished
sentence to the screen: it sends a code and data (`port_taken` with port and
owner, `below_min` with the minimum), and the panel writes it in the chosen
language. Only what docker itself writes, log lines and container status, shows
up as it came.

A password never enters `docker-compose.yml`: fields marked as secret go to an
`.env` next to it, with `0600` permissions and out of version control.

## An instance directory

```
<instance folder>/smp-family/
├── docker-compose.yml    generated, this is what Docker reads, and it is
│                         the whole instance: the okdock.* labels carry the
│                         template, the secret key names and the port names
├── .env                  secrets only, 0600, never versioned
└── data/                 the world
```

The panel configuration (the duckdns token, the domain links and the folders
chosen on the settings screen) lives outside of that, in `.okdock/` inside
`OKDOCK_CONFIG`, with `0600`. In the deploy that is `config/` next to the
compose file of the panel, never the instance folder: it is the file that records where
the instances went, so it cannot live where they are.

Changing the instance folder does not move what already exists: docker keeps the absolute path of the bind mounts, so old instances stay
up where they are and come back to the list if the root comes back. Inside the
container, only a path mounted there under the same name is worth anything.

The directory name, the compose project `name:` and the container name are
always the same text. When they diverge, `docker compose` acts on a project
other than the one the folder suggests.

## Documentation

- [`docs/architecture.md`](docs/architecture.md): how the layers fit together
  and why
- [`docs/api.md`](docs/api.md): REST + SSE contract
- [`docs/templates.md`](docs/templates.md): the builtin templates and how to
  write another

## Coming from GameDock

The project used to be called GameDock. The panel still reads whatever kept the
old name, so an existing installation comes up with no manual step:

| Before | Now | How it is read |
|---|---|---|
| `GAMEDOCK_*` | `OKDOCK_*` | the old variable counts when the new one is not set, with a warning in the log |
| `<root>/.gamedock/` | `<root>/.okdock/` | the old folder is read when the new one does not exist |
| `.okdock.json` in the instance | `okdock.*` labels in the compose | the old file is read when the compose file cannot be parsed, and is never written again |
| `gamedock.locale`, `gamedock.metrics` | `okdock.*` | the old browser key migrates on the first read |

What does **not** rename itself: the directory and the `container_name` of
instances already created keep the text you gave them, and the
`gamedock.managed` label stays on old containers until the next recreate.
Neither changes behavior, because the panel finds the instance by its directory.

## Planned improvements

Work written down, not implemented yet.

- Mod support: find out somehow whether the image takes mods and, when it does,
  show a **Mods** tab where loose files or a `.zip` can be dropped.
