package compose_test

import (
	"strings"
	"testing"

	"github.com/VitorCdSouza/okdock/api/internal/compose"
	"github.com/VitorCdSouza/okdock/api/internal/instance"
)

func TestApplyKeepsTheRestOfTheFile(t *testing.T) {
	p, err := compose.Parse([]byte(stack))
	if err != nil {
		t.Fatal(err)
	}
	app, _ := p.Service("nextcloud")
	spec := app.Spec()
	spec.Image = "nextcloud:30-apache"
	spec.MemoryLimit = "3G"
	spec.Ports = []instance.PortBinding{{Host: 8081, Container: 80, Protocol: "tcp"}}
	spec.Env["MYSQL_HOST"] = "db"

	if err := p.Apply("nextcloud", spec); err != nil {
		t.Fatal(err)
	}
	out, err := p.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	text := string(out)

	for _, want := range []string{
		"# nextcloud, written by hand",
		"container_name: nextcloud-cron",
		"entrypoint: /cron.sh",
		"mem_limit: 512M",
		"image: nextcloud:30-apache",
		`"8081:80/tcp"`,
		"memory: 3G",
		"MYSQL_HOST: db",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("%q is not in the file:\n%s", want, text)
		}
	}
	if strings.Contains(text, "8080:80") {
		t.Fatalf("the old port survived:\n%s", text)
	}
	// the value the author pointed at the .env round trips untouched
	if !strings.Contains(text, "${MYSQL_PASSWORD}") {
		t.Fatalf("the env reference was resolved:\n%s", text)
	}

	back, err := compose.Parse(out)
	if err != nil {
		t.Fatal(err)
	}
	again, ok := back.Service("nextcloud")
	if !ok {
		t.Fatal("the service is gone")
	}
	got := again.Spec()
	if got.Image != "nextcloud:30-apache" || got.MemoryLimit != "3G" {
		t.Fatalf("reread = %+v", got)
	}
	if len(got.Ports) != 1 || got.Ports[0].Host != 8081 {
		t.Fatalf("ports = %+v", got.Ports)
	}
	if len(got.Mounts) != 2 {
		t.Fatalf("the long form volume was lost: %+v", got.Mounts)
	}
}

func TestApplyKeepsTheFileDialect(t *testing.T) {
	p, err := compose.Parse([]byte(stack))
	if err != nil {
		t.Fatal(err)
	}
	cron, _ := p.Service("nextcloud-cron")
	spec := cron.Spec()
	spec.MemoryLimit = "256M"
	spec.Env["TZ"] = "UTC"

	if err := p.Apply("nextcloud-cron", spec); err != nil {
		t.Fatal(err)
	}
	out, _ := p.Bytes()
	text := string(out)
	if !strings.Contains(text, "mem_limit: 256M") {
		t.Fatalf("mem_limit turned into deploy:\n%s", text)
	}
	if strings.Contains(text, "deploy:\n      resources:\n        limits:\n          memory: 256M") {
		t.Fatalf("a second dialect was written:\n%s", text)
	}
	// the list stays a list, the mapping stays a mapping
	if !strings.Contains(text, "- TZ=UTC") {
		t.Fatalf("the KEY=value list became a mapping:\n%s", text)
	}
}

func TestApplyDropsTheSecretAndTheEmptyLimit(t *testing.T) {
	p, err := compose.Parse([]byte(`services:
  web:
    image: nginx
    environment:
      PASSWORD: sitting-here
      TZ: UTC
    deploy:
      resources:
        limits:
          memory: 1G
`))
	if err != nil {
		t.Fatal(err)
	}
	svc, _ := p.Service("web")
	spec := svc.Spec()
	spec.SecretKeys = []string{"PASSWORD"}
	spec.MemoryLimit = ""

	if err := p.Apply("web", spec); err != nil {
		t.Fatal(err)
	}
	out, _ := p.Bytes()
	text := string(out)
	if strings.Contains(text, "sitting-here") {
		t.Fatalf("the password stayed in the yaml:\n%s", text)
	}
	if strings.Contains(text, "memory:") {
		t.Fatalf("the cleared limit stayed:\n%s", text)
	}
	if !strings.Contains(text, "TZ: UTC") {
		t.Fatalf("the rest of the environment went away:\n%s", text)
	}
}

func TestApplyRefusesAServiceThatIsNotThere(t *testing.T) {
	p, err := compose.Parse([]byte("services:\n  web:\n    image: nginx\n"))
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Apply("other", instance.Spec{}); err == nil {
		t.Fatal("applying to a missing service should fail")
	}
}
