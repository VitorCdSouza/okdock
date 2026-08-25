package compose

import (
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/VitorCdSouza/okdock/api/internal/instance"
)

func spec() instance.Spec {
	return instance.Spec{
		Name:       "smp-familia",
		TemplateID: "minecraft-java",
		Category:   "games",
		Image:      "itzg/minecraft-server:java21",
		Env: map[string]string{
			"EULA":        "true",
			"MAX_PLAYERS": "10",
			"RCON_SENHA":  "hunter2",
		},
		SecretKeys: []string{"RCON_SENHA"},
		Ports: []instance.PortBinding{
			{Host: 25565, Container: 25565, Protocol: "tcp", Label: "Jogo"},
		},
		Mounts:           []instance.Mount{{Host: "./data", Container: "/data"}},
		MemoryLimit:      "6g",
		CPUs:             2,
		Restart:          "unless-stopped",
		StopGraceSeconds: 120,
		UpdatedAt:        time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC),
	}
}

func render(t *testing.T, s instance.Spec) map[string]any {
	t.Helper()
	raw, err := Render(s)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	var doc map[string]any
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("the generated compose is not valid YAML: %v\n%s", err, raw)
	}
	return doc
}

func TestRenderProjectNameMatchesDirName(t *testing.T) {
	doc := render(t, spec())
	if doc["name"] != "smp-familia" {
		t.Errorf("name = %v, queria smp-familia", doc["name"])
	}
	services, _ := doc["services"].(map[string]any)
	if _, ok := services["smp-familia"]; !ok {
		t.Errorf("the service is not named after the instance: %v", services)
	}
}

func TestRenderKeepsSecretsOutOfCompose(t *testing.T) {
	raw, err := Render(spec())
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if strings.Contains(string(raw), "hunter2") {
		t.Fatalf("senha vazou para o docker-compose.yml:\n%s", raw)
	}
	if !strings.Contains(string(raw), "env_file") {
		t.Errorf("compose com segredo precisa referenciar o .env:\n%s", raw)
	}

	env := RenderEnv(spec())
	if !strings.Contains(string(env), "RCON_SENHA=hunter2") {
		t.Errorf(".env did not get the secret:\n%s", env)
	}
	if strings.Contains(string(env), "MAX_PLAYERS") {
		t.Errorf("a non-secret value ended up in .env:\n%s", env)
	}
}

func TestRenderNoEnvFileWhenNoSecrets(t *testing.T) {
	s := spec()
	delete(s.Env, "RCON_SENHA")
	s.SecretKeys = nil

	raw, _ := Render(s)
	if strings.Contains(string(raw), "env_file") {
		t.Errorf("with no secret there should be no env_file:\n%s", raw)
	}
	if RenderEnv(s) != nil {
		t.Error("with no secret the .env should not be created")
	}
}

func TestRenderEmptySecretDoesNotCreateEnvFile(t *testing.T) {
	s := spec()
	s.Env["RCON_SENHA"] = ""

	raw, _ := Render(s)
	if strings.Contains(string(raw), "env_file") {
		t.Errorf("a blank password does not justify env_file:\n%s", raw)
	}
}

func TestRenderPortsAndLimits(t *testing.T) {
	doc := render(t, spec())
	svc := doc["services"].(map[string]any)["smp-familia"].(map[string]any)

	ports, _ := svc["ports"].([]any)
	if len(ports) != 1 || ports[0] != "25565:25565/tcp" {
		t.Errorf("ports = %v", ports)
	}
	if svc["stop_grace_period"] != "120s" {
		t.Errorf("stop_grace_period = %v; save de mundo precisa de tempo antes do SIGKILL", svc["stop_grace_period"])
	}
	limits := svc["deploy"].(map[string]any)["resources"].(map[string]any)["limits"].(map[string]any)
	if limits["memory"] != "6g" {
		t.Errorf("memory limit = %v", limits["memory"])
	}
}

func TestRenderQuotesPorts(t *testing.T) {
	raw, err := Render(spec())
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(string(raw), `"25565:25565/tcp"`) {
		t.Errorf("portas deviam sair entre aspas:\n%s", raw)
	}
}

func TestRenderRejectsBadName(t *testing.T) {
	s := spec()
	s.Name = "Name With Space"
	if _, err := Render(s); err == nil {
		t.Error("an invalid name should fail before any file is written")
	}
}

func TestRenderRejectsEmptyImage(t *testing.T) {
	s := spec()
	s.Image = ""
	if _, err := Render(s); err == nil {
		t.Error("imagem vazia devia falhar")
	}
}

func TestEnvEscaping(t *testing.T) {
	s := spec()
	s.Env["RCON_SENHA"] = `a b "c" $d`
	got := string(RenderEnv(s))
	want := `RCON_SENHA="a b \"c\" \$d"`
	if !strings.Contains(got, want) {
		t.Errorf(".env mal escapado.\nqueria conter: %s\nveio:\n%s", want, got)
	}
}

func TestRenderIsDeterministic(t *testing.T) {
	a, _ := Render(spec())
	b, _ := Render(spec())
	if string(a) != string(b) {
		t.Error("Render is not deterministic")
	}
}

func TestTheLabelsCarryWhatTheSchemaHasNoFieldFor(t *testing.T) {
	spec := spec()
	spec.Archived = true
	spec.CreatedAt = time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)

	raw, err := Render(spec)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	project, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	svc, ok := project.Service(spec.Name)
	if !ok {
		t.Fatalf("the service did not come back: %+v", project)
	}

	got := svc.Spec()
	if len(got.SecretKeys) != 1 || got.SecretKeys[0] != "RCON_SENHA" {
		t.Errorf("secretKeys = %v", got.SecretKeys)
	}
	if !got.Archived {
		t.Error("archived was lost")
	}
	if !got.CreatedAt.Equal(spec.CreatedAt) {
		t.Errorf("createdAt = %v, wanted %v", got.CreatedAt, spec.CreatedAt)
	}
	if len(got.Ports) != 1 || got.Ports[0].Label != "Jogo" {
		t.Errorf("the name of the port was lost: %+v", got.Ports)
	}
}

func TestAPortAddedByHandKeepsTheNameOfTheOthers(t *testing.T) {
	spec := spec()
	spec.Ports = append(spec.Ports,
		instance.PortBinding{Host: 25575, Container: 25575, Protocol: "tcp", Label: "rcon"})
	raw, err := Render(spec)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	// a port with no label beside it does not steal the name of the one that has one
	edited := strings.Replace(string(raw), `      - "25565:25565/tcp"`,
		"      - \"25565:25565/tcp\"\n      - \"8080:8080/tcp\"", 1)
	project, err := Parse([]byte(edited))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	svc, _ := project.Service(spec.Name)
	byPort := map[int]string{}
	for _, p := range svc.Spec().Ports {
		byPort[p.Container] = p.Label
	}
	if byPort[25565] != "Jogo" || byPort[25575] != "rcon" || byPort[8080] != "" {
		t.Fatalf("labels = %v", byPort)
	}
}
