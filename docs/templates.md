# Templates

A **template** is a Docker image plus the schema of the fields it accepts. This
is where the wizard gets its form, since the frontend knows no variable of any
image.

A template is JSON. The ones shipping with OkDock live in
[`api/internal/template/builtin/`](../api/internal/template/builtin/), embedded
in the binary through `go:embed`. The ones registered through the **New
template** screen go to the templates folder as `<id>.json`, which is
`<boot root>/.okdock/templates/` until another one is chosen in the settings
screen, and a file
with the same id beats the builtin one: that is how a builtin template is edited
without losing the original, which comes back when the edit is deleted.

## Categories

Six ship with the panel (`games`, `media`, `database`, `network`, `utilities`,
`other`), each with its own color, icon and translation. A template may also
carry a category of its own, any slug matching `^[a-z0-9][a-z0-9-]{1,31}$`: the
**+ Category** button on the Templates screen writes one, the icon falls back to
the color of `other` and the name shows up as the slug itself until it gets a
`category.<slug>` key in `messages.*.ts`. That category exists while some
template uses it, because `GET /templates` builds the list out of the catalog;
a category left with no template is gone on the next load. To make one of them
shipped, add a constant in `internal/template/template.go` and the two keys in
`messages.*.ts`.

## What ships with the panel

| Template | Id | Category | Image | Default / minimum RAM | Ports |
|---|---|---|---|---|---|
| Minecraft (Java) | `minecraft-java` | games | `itzg/minecraft-server:java21` | 4g / 2g | 25565/tcp |
| Terraria (TShock) | `terraria-tshock` | games | `ryshe/terraria:tshock-1.4.5.6-6.1.0` | 2g / 512m | 7777/tcp |
| Terraria (vanilla) | `terraria-vanilla` | games | `ryshe/terraria:vanilla-1.4.5.7` | 2g / 512m | 7777/tcp |
| Custom image | `custom` | other | anything | 2g / 256m | you define |

The id is the file name on disk: lowercase, digits and hyphen. An instance
created before that saved the id with a slash (`itzg/minecraft-server`), and the
catalog lookup translates those three old ids.

## The two Terraria variants

The Terraria client refuses to join a server on a different version: *"You are
not using the same version as this server"*. That matters because the two images
move at different speeds.

- **TShock** brings plugins, permissions and admin commands, but takes weeks to
  catch up with a Terraria release.
- **Vanilla** has none of that and ships in days.

When the game updates and the client stops connecting, the way out is the
vanilla variant, or waiting for TShock.

The two templates exist separately because they are **not the same image with
another tag**: the `bootstrap.sh` differs. The TShock one accepts
`WORLD_FILENAME` and creates the world if `-autocreate` is in the arguments. The
vanilla one **exits with an error** if `WORLD_FILENAME` is filled and the world
does not exist; there the world path goes through `-world`, and
`WORLD_FILENAME` stays empty.

Switching only the tag of a TShock instance to vanilla therefore does not work:
the container dies at boot and, with `restart: unless-stopped`, enters a
crashloop. Nothing in the panel refuses that combination: a template no longer
declares which images it accepts, so the pairing is on whoever fills the Image
field. To migrate between variants, create another instance pointing at the
same world folder.

## Pinned tags

The game images use a version tag, never `:latest`. A moving tag changes the
server version on its own at the next recreate, and the symptom shows up far
from the cause: the player is the one who finds out, by failing to join.
`TestBuiltinImagesArePinned` fails if anyone reintroduces one.

Updating a version means changing the **Image** field of the instance in the
panel and saving; the world in the volumes is preserved.

`freeEnv: true` is what lets a template accept a variable outside the schema;
`custom` is the only builtin with it on, and it is the way out for an image no
template describes.

## Verification state of the schemas

The two game templates were exercised against a real container on 2026-08-21:
create the instance through the panel, bring it up, and confirm the
configuration took. Minecraft generates the world and accepts a connection;
Terraria does the same, with `okdock.wld` created by `-autocreate`.

The catalog once had six other games (Minecraft Bedrock, Palworld, Valheim, ARK,
Factorio, Satisfactory), removed on 2026-08-21 at the user request. Their
schemas are in the git history, in `internal/catalog/providers.go` before the
commit that took them out. It is worth recovering them from there instead of
rewriting from scratch, but **without trusting them**: only Minecraft and
Terraria were actually run, and the Terraria case showed the mistake may not be
a wrong variable name but the whole mechanism being wrong.

When writing a template for a new image, the routine that worked was:

1. `skopeo inspect --config docker://<image>`, which shows the entrypoint and
   the declared variables without pulling the image.
2. Read the entrypoint (`docker run --rm --entrypoint cat <image> /path/to/script`),
   which is what reveals whether the image reads configuration from the
   environment or from arguments.
3. Bring an instance up and confirm the configuration had an effect.

Step 1 alone misleads: **the list of declared variables is not the list of
variables read.** Many images use `${VAR:-default}` in the script without
declaring anything in `ENV`. The inspection proves what exists, never what does
not.

A field with the wrong name does not break the `up`: the variable is ignored and
the effect is the configuration not taking. Configuration the image expects as
an **argument** is worse: the symptom is a container that comes up and does
nothing, as happened with Terraria before `TargetArg`.

None of this stops anyone from using an image outside the catalog: the `custom`
template always accepts the image with the variables typed by hand.

## Adding a template

Through the panel: **New template**, at the top. It is written to the templates
folder and shows up in the wizard right away, nothing has to be recompiled.

To add a template that ships with OkDock:

1. Add the `.json` to `internal/template/builtin/`, with the id equal to the
   file name.
2. `go test ./internal/template/`. `TestAllBuiltinTemplatesAreUsable` already
   covers the basics: full identification, no repeated keys, enums with options, and **every default passing its own validation** (a
   default outside the schema would make the instance be born invalid without
   the user touching anything). `Template.Check` runs at load time, so invalid
   JSON takes the boot down instead of showing up broken on screen.
3. Nothing changes in the frontend.

### A field that becomes an argument, not a variable

`Target: TargetArg` plus `Flag: "-autocreate"` sends the value to the service
`command:` instead of the environment. A boolean field emits only the flag when
true, with no value. The argument order follows the field order in the template,
not the map order, so the generated compose does not change on every render.

A secret field that is an argument becomes `${KEY}` in the compose, and the
value goes to the `.env`: `docker compose` interpolates it by reading the `.env`
of the project directory. That way the password does not show up in the YAML
even when it is passed on the command line.

The fields that deserve attention:

- **`secret: true`** on any password. It is what keeps the value out of
  `docker-compose.yml`.
- **An honest `minMemory`**. It is what the panel uses to refuse an instance
  that would die with `Exited (137)`.
- **A generous `stopGraceSeconds`** on a game that saves on shutdown. Minecraft
  and ARK corrupt the save if they take a SIGKILL mid-write; 120 s and 180 s are
  the values used today.
- **`optional: true`** on RCON and query ports. They are not published by
  default, and publishing RCON without a reason is opening a remote console to
  the server.
- **`advanced: true`** on what almost nobody touches, so the form does not scare
  anyone.

## How validation works

`Template.Validate` applies the defaults, checks type, range and enum, and
gathers **every** problem before returning, so the form marks the wrong fields
all at once instead of forcing several saves to discover the rest.
