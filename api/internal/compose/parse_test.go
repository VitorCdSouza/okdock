package compose_test

import (
	"strings"
	"testing"

	"github.com/VitorCdSouza/okdock/api/internal/compose"
	"github.com/VitorCdSouza/okdock/api/internal/instance"
)

const stack = `# nextcloud, written by hand
name: nextcloud
services:
  app:
    image: nextcloud:apache
    container_name: nextcloud
    restart: unless-stopped
    ports:
      - "8080:80"
      - "1935:1935/udp"
    environment:
      MYSQL_HOST: nextcloud-mysql
      MYSQL_PASSWORD: ${MYSQL_PASSWORD}
    volumes:
      - /srv/nextcloud/html:/var/www/html
      - type: bind
        source: /srv/nextcloud/data
        target: /data
    deploy:
      resources:
        limits:
          memory: 2G
          cpus: "1.5"
  cron:
    image: nextcloud:apache
    container_name: nextcloud-cron
    entrypoint: /cron.sh
    environment:
      - TZ=America/Sao_Paulo
      - DEBUG=
    mem_limit: 512M
    command: php -f /var/www/html/cron.php
`

func TestParseReadsBothDialects(t *testing.T) {
	p, err := compose.Parse([]byte(stack))
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "nextcloud" {
		t.Fatalf("project name = %q", p.Name)
	}
	if len(p.Services) != 2 {
		t.Fatalf("services = %d, want 2", len(p.Services))
	}
	if len(p.Unsupported) != 0 {
		t.Fatalf("unsupported = %v", p.Unsupported)
	}

	app, ok := p.Service("nextcloud")
	if !ok {
		t.Fatal("the container_name did not find the service")
	}
	spec := app.Spec()
	if spec.Name != "nextcloud" || spec.Image != "nextcloud:apache" || spec.Restart != "unless-stopped" {
		t.Fatalf("spec = %+v", spec)
	}
	if spec.MemoryLimit != "2G" || spec.CPUs != 1.5 {
		t.Fatalf("limits = %q %v", spec.MemoryLimit, spec.CPUs)
	}
	want := []instance.PortBinding{
		{Host: 8080, Container: 80, Protocol: "tcp"},
		{Host: 1935, Container: 1935, Protocol: "udp"},
	}
	if len(spec.Ports) != len(want) {
		t.Fatalf("ports = %+v", spec.Ports)
	}
	for i := range want {
		if spec.Ports[i] != want[i] {
			t.Fatalf("port %d = %+v, want %+v", i, spec.Ports[i], want[i])
		}
	}
	if len(spec.Mounts) != 2 || spec.Mounts[1].Host != "/srv/nextcloud/data" || spec.Mounts[1].Container != "/data" {
		t.Fatalf("mounts = %+v", spec.Mounts)
	}
	// the value that points at the .env is read as written, not resolved
	if spec.Env["MYSQL_PASSWORD"] != "${MYSQL_PASSWORD}" {
		t.Fatalf("env = %v", spec.Env)
	}

	cron, _ := p.Service("cron")
	cronSpec := cron.Spec()
	if cronSpec.Name != "nextcloud-cron" || cronSpec.MemoryLimit != "512M" {
		t.Fatalf("cron spec = %+v", cronSpec)
	}
	if cronSpec.Env["TZ"] != "America/Sao_Paulo" || cronSpec.Env["DEBUG"] != "" {
		t.Fatalf("cron env = %v", cronSpec.Env)
	}
	if strings.Join(cronSpec.Command, " ") != "php -f /var/www/html/cron.php" {
		t.Fatalf("command = %v", cronSpec.Command)
	}
}

func TestParseReadsWhatRenderWrote(t *testing.T) {
	spec := instance.Spec{
		Name:             "minecraft",
		TemplateID:       "itzg/minecraft-server",
		Category:         "games",
		Image:            "itzg/minecraft-server:java21",
		Env:              map[string]string{"EULA": "TRUE", "RCON_PASSWORD": "hunter2"},
		SecretKeys:       []string{"RCON_PASSWORD"},
		Ports:            []instance.PortBinding{{Host: 25565, Container: 25565, Protocol: "tcp"}},
		Mounts:           []instance.Mount{{Host: "./data", Container: "/data", Data: true}},
		MemoryLimit:      "4G",
		CPUs:             2,
		Restart:          "unless-stopped",
		StopGraceSeconds: 120,
	}
	yml, err := compose.Render(spec)
	if err != nil {
		t.Fatal(err)
	}
	p, err := compose.Parse(yml)
	if err != nil {
		t.Fatal(err)
	}
	svc, ok := p.Service("minecraft")
	if !ok {
		t.Fatal("the rendered service did not come back")
	}
	got := svc.Spec()
	if got.Name != spec.Name || got.Image != spec.Image {
		t.Fatalf("got = %+v", got)
	}
	if got.TemplateID != spec.TemplateID || got.Category != spec.Category {
		t.Fatalf("the label did not carry the template: %+v", got)
	}
	if got.MemoryLimit != "4G" || got.CPUs != 2 || got.Restart != "unless-stopped" {
		t.Fatalf("limits = %+v", got)
	}
	if got.StopGraceSeconds != 120 {
		t.Fatalf("stop grace = %d", got.StopGraceSeconds)
	}
	if got.Env["EULA"] != "TRUE" {
		t.Fatalf("env = %v", got.Env)
	}
	// the secret never reaches the yaml, so reading it back cannot see it
	if _, ok := got.Env["RCON_PASSWORD"]; ok {
		t.Fatal("the password leaked into the compose file")
	}
	if len(got.Ports) != 1 || got.Ports[0].Host != 25565 {
		t.Fatalf("ports = %+v", got.Ports)
	}
	if len(got.Mounts) != 1 || got.Mounts[0].Container != "/data" {
		t.Fatalf("mounts = %+v", got.Mounts)
	}
}

func TestParseFlagsWhatItCannotEdit(t *testing.T) {
	p, err := compose.Parse([]byte(`
include:
  - other.yml
services:
  web:
    image: nginx
    ports:
      - "8000-8005:8000-8005"
`))
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Unsupported) != 2 {
		t.Fatalf("unsupported = %v, want include and the port range", p.Unsupported)
	}
}
