package compose

import (
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/VitorCdSouza/gamedock/api/internal/instance"
)

func spec() instance.Spec {
	return instance.Spec{
		Name:       "smp-familia",
		ProviderID: "itzg/minecraft-server",
		Game:       "minecraft-java",
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
		Mounts:           []instance.Mount{{Host: "./data", Container: "/data", Data: true}},
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
		t.Fatalf("compose gerado não é YAML válido: %v\n%s", err, raw)
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
		t.Errorf("serviço não tem o nome da instância: %v", services)
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
		t.Errorf(".env não recebeu o segredo:\n%s", env)
	}
	if strings.Contains(string(env), "MAX_PLAYERS") {
		t.Errorf("valor não secreto foi parar no .env:\n%s", env)
	}
}

func TestRenderNoEnvFileWhenNoSecrets(t *testing.T) {
	s := spec()
	delete(s.Env, "RCON_SENHA")
	s.SecretKeys = nil

	raw, _ := Render(s)
	if strings.Contains(string(raw), "env_file") {
		t.Errorf("sem segredo não devia haver env_file:\n%s", raw)
	}
	if RenderEnv(s) != nil {
		t.Error("sem segredo o .env não devia ser criado")
	}
}

func TestRenderEmptySecretDoesNotCreateEnvFile(t *testing.T) {
	s := spec()
	s.Env["RCON_SENHA"] = ""

	raw, _ := Render(s)
	if strings.Contains(string(raw), "env_file") {
		t.Errorf("senha em branco não justifica env_file:\n%s", raw)
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
		t.Errorf("limite de memória = %v", limits["memory"])
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
	s.Name = "Nome Com Espaço"
	if _, err := Render(s); err == nil {
		t.Error("nome inválido devia falhar antes de gerar arquivo")
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
		t.Error("Render não é determinístico")
	}
}
