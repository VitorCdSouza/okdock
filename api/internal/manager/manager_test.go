package manager

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/VitorCdSouza/okdock/api/internal/dockerx"
	"github.com/VitorCdSouza/okdock/api/internal/instance"
	"github.com/VitorCdSouza/okdock/api/internal/store"
	"github.com/VitorCdSouza/okdock/api/internal/system"
	"github.com/VitorCdSouza/okdock/api/internal/template"
)

// templates devolve o catalogo de fabrica, com diretorio temporario e vazio no teste
func templates(t *testing.T) *template.Catalog {
	t.Helper()
	c, err := template.NewCatalog(t.TempDir())
	if err != nil {
		t.Fatalf("NewCatalog: %v", err)
	}
	return c
}

func newManager(t *testing.T, totalRAM int64) (*Manager, *dockerx.Fake) {
	t.Helper()
	st, err := store.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	fake := dockerx.NewFake()
	m := New(Options{
		Store:     st,
		Templates: templates(t),
		Docker:    fake,
		System: system.StaticReader{Info: system.Info{
			MemoryTotal:     totalRAM,
			MemoryAvailable: totalRAM,
			CPUCount:        8,
		}},
		MemoryReserve: 2 << 30,
	})
	return m, fake
}

const gb = int64(1) << 30

func req(name, mem string) SpecRequest {
	return SpecRequest{
		Name:        name,
		TemplateID:  "minecraft-java",
		Values:      map[string]string{"EULA": "true"},
		MemoryLimit: mem,
	}
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timeout esperando: %s", what)
}

func TestBuildSpecUsesProviderDefaults(t *testing.T) {
	m, _ := newManager(t, 16*gb)
	spec, err := m.BuildSpec(req("smp", ""))
	if err != nil {
		t.Fatalf("BuildSpec: %v", err)
	}
	if spec.Image != "itzg/minecraft-server:java21" {
		t.Errorf("imagem = %q", spec.Image)
	}
	if spec.MemoryLimit != "4g" {
		t.Errorf("limite de RAM = %q, queria o default do provedor", spec.MemoryLimit)
	}
	if len(spec.Ports) != 1 || spec.Ports[0].Host != 25565 {
		t.Errorf("portas = %v", spec.Ports)
	}
	if spec.Env["TYPE"] != "VANILLA" {
		t.Errorf("default de campo não aplicado: %v", spec.Env)
	}
}

func TestBuildSpecMarksProviderSecrets(t *testing.T) {
	m, _ := newManager(t, 32*gb)
	spec, err := m.BuildSpec(SpecRequest{
		Name:       "terraria-1",
		TemplateID: "terraria-tshock",
		Values:     map[string]string{"PASSWORD": "abc"},
	})
	if err != nil {
		t.Fatalf("BuildSpec: %v", err)
	}
	found := false
	for _, k := range spec.SecretKeys {
		if k == "PASSWORD" {
			found = true
		}
	}
	if !found {
		t.Errorf("PASSWORD não foi marcada como segredo: %v", spec.SecretKeys)
	}
	for _, a := range spec.Command {
		if a == "abc" {
			t.Errorf("senha vazou para o comando: %v", spec.Command)
		}
	}
}

func TestBuildSpecRejectsMemoryBelowProviderMinimum(t *testing.T) {
	m, _ := newManager(t, 32*gb)
	_, err := m.BuildSpec(req("smp", "512m"))
	if err == nil {
		t.Fatal("RAM abaixo do mínimo do provedor devia falhar")
	}
	if !strings.Contains(err.Error(), "137") {
		t.Errorf("erro devia explicar o sintoma (exit 137): %v", err)
	}
}

func TestBuildSpecCustomNeedsImage(t *testing.T) {
	m, _ := newManager(t, 32*gb)
	_, err := m.BuildSpec(SpecRequest{Name: "custom-1", TemplateID: "custom"})
	if err == nil {
		t.Fatal("provedor custom sem imagem devia falhar")
	}
}

func TestCreateWithoutStartDoesNotTouchDocker(t *testing.T) {
	m, fake := newManager(t, 16*gb)
	if _, err := m.Create(context.Background(), req("smp", "4g")); err != nil {
		t.Fatalf("Create: %v", err)
	}
	for _, c := range fake.Calls {
		if strings.HasPrefix(c, "up:") || strings.HasPrefix(c, "pull:") {
			t.Errorf("criar sem start não devia subir nada; chamadas: %v", fake.Calls)
		}
	}
	inst, err := m.Get(context.Background(), "smp")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if inst.State != instance.StateStopped {
		t.Errorf("estado = %q, queria stopped", inst.State)
	}
}

func TestCreateWithStartPullsThenUps(t *testing.T) {
	m, fake := newManager(t, 16*gb)
	r := req("smp", "4g")
	r.Start = true
	if _, err := m.Create(context.Background(), r); err != nil {
		t.Fatalf("Create: %v", err)
	}
	waitFor(t, "instância subir", func() bool {
		i, err := m.Get(context.Background(), "smp")
		return err == nil && i.State == instance.StateRunning
	})

	var order []string
	for _, c := range fake.Calls {
		if strings.HasPrefix(c, "pull:") || strings.HasPrefix(c, "up:") {
			order = append(order, strings.SplitN(c, ":", 2)[0])
		}
	}
	if len(order) < 2 || order[0] != "pull" || order[1] != "up" {
		t.Errorf("ordem das chamadas = %v", order)
	}
}

func TestCreateRejectsPortAlreadyUsed(t *testing.T) {
	m, _ := newManager(t, 32*gb)
	first := req("mc-a", "4g")
	first.Ports = []instance.PortBinding{{Host: 25565, Container: 25565, Protocol: "tcp"}}
	if _, err := m.Create(context.Background(), first); err != nil {
		t.Fatalf("Create: %v", err)
	}

	second := req("mc-b", "4g")
	second.Ports = []instance.PortBinding{{Host: 25565, Container: 25565, Protocol: "tcp"}}
	_, err := m.Create(context.Background(), second)

	var pe *ErrPortTaken
	if !errors.As(err, &pe) {
		t.Fatalf("esperava ErrPortTaken, veio %v", err)
	}
	if pe.Owner != "mc-a" {
		t.Errorf("erro devia apontar quem ocupa a porta, veio %q", pe.Owner)
	}
}

func TestSuggestPortSkipsTakenOnes(t *testing.T) {
	m, _ := newManager(t, 32*gb)
	if _, err := m.Create(context.Background(), req("mc-a", "4g")); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if got := m.SuggestPort(25565, "tcp"); got != 25566 {
		t.Errorf("SuggestPort = %d, queria 25566", got)
	}
}

func TestStartRefusesWhenRAMBudgetIsExceeded(t *testing.T) {
	m, _ := newManager(t, 8*gb)

	up := req("mc-a", "4g")
	up.Start = true
	if _, err := m.Create(context.Background(), up); err != nil {
		t.Fatalf("Create: %v", err)
	}
	waitFor(t, "mc-a subir", func() bool {
		i, _ := m.Get(context.Background(), "mc-a")
		return i.State == instance.StateRunning
	})

	if _, err := m.Create(context.Background(), req("mc-b", "4g")); err != nil {
		t.Fatalf("Create: %v", err)
	}
	err := m.Start(context.Background(), "mc-b")

	var be *ErrBudget
	if !errors.As(err, &be) {
		t.Fatalf("esperava ErrBudget, veio %v", err)
	}
	if be.Budget != 6*gb {
		t.Errorf("orçamento = %d, queria 6 GiB", be.Budget)
	}
}

func TestStoppedInstancesDoNotConsumeBudget(t *testing.T) {
	m, _ := newManager(t, 8*gb)
	if _, err := m.Create(context.Background(), req("mc-a", "5g")); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := m.Create(context.Background(), req("mc-b", "5g")); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := m.Start(context.Background(), "mc-b"); err != nil {
		t.Fatalf("instância parada não devia contar no orçamento: %v", err)
	}
}

func TestStopBringsInstanceDown(t *testing.T) {
	m, _ := newManager(t, 16*gb)
	r := req("smp", "4g")
	r.Start = true
	if _, err := m.Create(context.Background(), r); err != nil {
		t.Fatalf("Create: %v", err)
	}
	waitFor(t, "subir", func() bool {
		i, _ := m.Get(context.Background(), "smp")
		return i.State == instance.StateRunning
	})

	if err := m.Stop(context.Background(), "smp"); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	waitFor(t, "parar", func() bool {
		i, _ := m.Get(context.Background(), "smp")
		return i.State == instance.StateStopped
	})
}

func TestFailedOperationSurfacesAsError(t *testing.T) {
	m, fake := newManager(t, 16*gb)
	if _, err := m.Create(context.Background(), req("smp", "4g")); err != nil {
		t.Fatalf("Create: %v", err)
	}
	fake.FailUp[m.Store().Dir("smp")] = errors.New("port is already allocated")

	if err := m.Start(context.Background(), "smp"); err != nil {
		t.Fatalf("Start: %v", err)
	}
	waitFor(t, "falha aparecer", func() bool {
		i, _ := m.Get(context.Background(), "smp")
		return i.State == instance.StateError
	})

	inst, _ := m.Get(context.Background(), "smp")
	if !strings.Contains(inst.Status, "already allocated") {
		t.Errorf("o card precisa mostrar o motivo do docker, veio %q", inst.Status)
	}

	m.ClearError("smp")
	inst, _ = m.Get(context.Background(), "smp")
	if inst.State == instance.StateError {
		t.Error("ClearError não limpou o estado de erro")
	}
}

func TestUpdateRecreatesWhenEnvChanges(t *testing.T) {
	m, fake := newManager(t, 16*gb)
	r := req("smp", "4g")
	r.Start = true
	if _, err := m.Create(context.Background(), r); err != nil {
		t.Fatalf("Create: %v", err)
	}
	waitFor(t, "subir", func() bool {
		i, _ := m.Get(context.Background(), "smp")
		return i.State == instance.StateRunning
	})
	before := len(fake.Calls)

	next := req("smp", "4g")
	next.Values["MAX_PLAYERS"] = "30"
	if _, err := m.Update(context.Background(), "smp", next); err != nil {
		t.Fatalf("Update: %v", err)
	}
	waitFor(t, "recriar", func() bool {
		i, _ := m.Get(context.Background(), "smp")
		return i.State == instance.StateRunning
	})

	var down, up bool
	for _, c := range fake.Calls[before:] {
		if strings.HasPrefix(c, "down:") {
			down = true
		}
		if strings.HasPrefix(c, "up:") {
			up = true
		}
	}
	if !down || !up {
		t.Errorf("update de env devia derrubar e subir; chamadas: %v", fake.Calls[before:])
	}
}

func TestNeedsRecreate(t *testing.T) {
	base := instance.Spec{
		Image:       "img:1",
		Env:         map[string]string{"A": "1"},
		MemoryLimit: "4g",
		Ports:       []instance.PortBinding{{Host: 1, Container: 1, Protocol: "tcp"}},
	}
	same := instance.Spec{
		Image:       "img:1",
		Env:         map[string]string{"A": "1"},
		MemoryLimit: "4g",
		Ports:       []instance.PortBinding{{Host: 1, Container: 1, Protocol: "tcp"}},
	}
	if NeedsRecreate(base, same) {
		t.Error("specs iguais não deviam pedir recriação")
	}

	changed := same
	changed.Env = map[string]string{"A": "2"}
	if !NeedsRecreate(base, changed) {
		t.Error("mudança de env devia pedir recriação")
	}

	changed = same
	changed.MemoryLimit = "8g"
	if !NeedsRecreate(base, changed) {
		t.Error("mudança de limite de RAM devia pedir recriação")
	}
}

func TestDeleteRemovesInstance(t *testing.T) {
	m, _ := newManager(t, 16*gb)
	if _, err := m.Create(context.Background(), req("smp", "4g")); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := m.Delete(context.Background(), "smp", true); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := m.Get(context.Background(), "smp"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("esperava ErrNotFound, veio %v", err)
	}
}

func TestArchivedInstanceRefusesToStart(t *testing.T) {
	m, _ := newManager(t, 16*gb)
	if _, err := m.Create(context.Background(), req("smp", "4g")); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := m.SetArchived(context.Background(), "smp", true); err != nil {
		t.Fatalf("SetArchived: %v", err)
	}
	inst, _ := m.Get(context.Background(), "smp")
	if inst.State != instance.StateArchived {
		t.Errorf("estado = %q, queria archived", inst.State)
	}
	if err := m.Start(context.Background(), "smp"); err == nil {
		t.Error("instância arquivada não devia subir sem ser restaurada")
	}
}

func TestSystemReportsCommittedMemory(t *testing.T) {
	m, _ := newManager(t, 16*gb)
	r := req("smp", "4g")
	r.Start = true
	if _, err := m.Create(context.Background(), r); err != nil {
		t.Fatalf("Create: %v", err)
	}
	waitFor(t, "subir", func() bool {
		i, _ := m.Get(context.Background(), "smp")
		return i.State == instance.StateRunning
	})

	info, err := m.System(context.Background())
	if err != nil {
		t.Fatalf("System: %v", err)
	}
	if info.MemoryBudget != 14*gb {
		t.Errorf("orçamento = %d, queria 14 GiB", info.MemoryBudget)
	}
	if info.MemoryCommitted != 4*gb {
		t.Errorf("comprometido = %d, queria 4 GiB", info.MemoryCommitted)
	}
}

func TestDeriveState(t *testing.T) {
	cases := []struct {
		name       string
		containers []dockerx.Container
		want       instance.State
	}{
		{"sem container", nil, instance.StateStopped},
		{"rodando", []dockerx.Container{{State: "running"}}, instance.StateRunning},
		{"healthcheck ainda subindo", []dockerx.Container{{State: "running", Health: "starting"}}, instance.StateStarting},
		{"unhealthy", []dockerx.Container{{State: "running", Health: "unhealthy"}}, instance.StateError},
		{"saiu limpo", []dockerx.Container{{State: "exited", ExitCode: 0}}, instance.StateStopped},
		{"OOM killed", []dockerx.Container{{State: "exited", ExitCode: 137}}, instance.StateError},
		{"criado mas não iniciado", []dockerx.Container{{State: "created"}}, instance.StateProvisioning},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, _, _, _ := deriveState(instance.Spec{}, tc.containers, nil)
			if got != tc.want {
				t.Errorf("deriveState = %q, queria %q", got, tc.want)
			}
		})
	}
}

func TestHubDeliversAndUnsubscribes(t *testing.T) {
	h := NewHub()
	ch, cancel := h.Subscribe()

	h.Publish(Event{Type: "instance.changed", Instance: "smp"})
	select {
	case ev := <-ch:
		if ev.Instance != "smp" {
			t.Errorf("evento = %+v", ev)
		}
	case <-time.After(time.Second):
		t.Fatal("evento não chegou")
	}

	cancel()
	h.Publish(Event{Type: "instance.changed", Instance: "smp"})
	if _, open := <-ch; open {
		t.Error("canal devia estar fechado após cancel")
	}
}

func TestSlowSubscriberDoesNotBlockPublish(t *testing.T) {
	h := NewHub()
	_, cancel := h.Subscribe()
	defer cancel()

	done := make(chan struct{})
	go func() {
		for i := 0; i < 200; i++ {
			h.Publish(Event{Type: "instance.changed"})
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Publish travou num assinante lento")
	}
}

func TestUpdateImageRecreatesOnlyWhenTheImageChanged(t *testing.T) {
	m, fake := newManager(t, 16*gb)
	r := req("smp", "4g")
	r.Start = true
	if _, err := m.Create(context.Background(), r); err != nil {
		t.Fatalf("Create: %v", err)
	}
	waitFor(t, "subir", func() bool {
		i, _ := m.Get(context.Background(), "smp")
		return i.State == instance.StateRunning
	})

	image := "itzg/minecraft-server:java21"
	fake.ImageIDs[image] = "sha256:antiga"
	fake.PulledImageIDs[image] = "sha256:antiga"
	before := len(fake.Calls)

	if err := m.UpdateImage(context.Background(), "smp"); err != nil {
		t.Fatalf("UpdateImage: %v", err)
	}
	waitFor(t, "checagem terminar", func() bool {
		i, _ := m.Get(context.Background(), "smp")
		return i.Operation == nil
	})

	for _, c := range fake.Calls[before:] {
		if strings.HasPrefix(c, "up:") {
			t.Errorf("recriou sem imagem nova; chamadas: %v", fake.Calls[before:])
		}
	}

	fake.PulledImageIDs[image] = "sha256:nova"
	before = len(fake.Calls)
	if err := m.UpdateImage(context.Background(), "smp"); err != nil {
		t.Fatalf("UpdateImage: %v", err)
	}
	waitFor(t, "recriar", func() bool {
		i, _ := m.Get(context.Background(), "smp")
		return i.Operation == nil
	})

	var recreated bool
	for _, c := range fake.Calls[before:] {
		if strings.HasPrefix(c, "up:") {
			recreated = true
		}
	}
	if !recreated {
		t.Errorf("imagem nova devia recriar o container; chamadas: %v", fake.Calls[before:])
	}
}

func TestUpdateImagePublishesWhatHappened(t *testing.T) {
	m, fake := newManager(t, 16*gb)
	if _, err := m.Create(context.Background(), req("smp", "4g")); err != nil {
		t.Fatalf("Create: %v", err)
	}
	image := "itzg/minecraft-server:java21"
	fake.ImageIDs[image] = "sha256:antiga"
	fake.PulledImageIDs[image] = "sha256:antiga"

	events, cancel := m.Events().Subscribe()
	defer cancel()

	if err := m.UpdateImage(context.Background(), "smp"); err != nil {
		t.Fatalf("UpdateImage: %v", err)
	}

	deadline := time.After(3 * time.Second)
	for {
		select {
		case ev := <-events:
			if ev.Type == "instance.uptodate" && ev.Instance == "smp" {
				if !strings.Contains(ev.Message, "mais nova") {
					t.Errorf("mensagem pouco clara: %q", ev.Message)
				}
				return
			}
		case <-deadline:
			t.Fatal("nenhum evento dizendo que já estava atualizada")
		}
	}
}

func TestUpdateImageRefusesArchived(t *testing.T) {
	m, _ := newManager(t, 16*gb)
	if _, err := m.Create(context.Background(), req("smp", "4g")); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := m.SetArchived(context.Background(), "smp", true); err != nil {
		t.Fatalf("SetArchived: %v", err)
	}
	if err := m.UpdateImage(context.Background(), "smp"); err == nil {
		t.Error("instância arquivada não devia ser atualizada sem restaurar")
	}
}
