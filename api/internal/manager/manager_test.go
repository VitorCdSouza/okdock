package manager

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/VitorCdSouza/okdock/api/internal/dockerx"
	"github.com/VitorCdSouza/okdock/api/internal/instance"
	"github.com/VitorCdSouza/okdock/api/internal/store"
	"github.com/VitorCdSouza/okdock/api/internal/system"
	"github.com/VitorCdSouza/okdock/api/internal/template"
)

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
	t.Fatalf("timeout waiting for: %s", what)
}

func TestBuildSpecUsesProviderDefaults(t *testing.T) {
	m, _ := newManager(t, 16*gb)
	spec, err := m.BuildSpec(req("smp", ""))
	if err != nil {
		t.Fatalf("BuildSpec: %v", err)
	}
	if spec.Image != "itzg/minecraft-server:java21" {
		t.Errorf("image = %q", spec.Image)
	}
	if spec.MemoryLimit != "4g" {
		t.Errorf("RAM limit = %q, wanted the template default", spec.MemoryLimit)
	}
	if len(spec.Ports) != 1 || spec.Ports[0].Host != 25565 {
		t.Errorf("ports = %v", spec.Ports)
	}
	if spec.Env["TYPE"] != "VANILLA" {
		t.Errorf("field default not applied: %v", spec.Env)
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
		t.Errorf("PASSWORD was not marked as a secret: %v", spec.SecretKeys)
	}
	for _, a := range spec.Command {
		if a == "abc" {
			t.Errorf("the password leaked into the command: %v", spec.Command)
		}
	}
}

func TestBuildSpecRejectsMemoryBelowProviderMinimum(t *testing.T) {
	m, _ := newManager(t, 32*gb)
	_, err := m.BuildSpec(req("smp", "512m"))
	if err == nil {
		t.Fatal("RAM below the template minimum should fail")
	}
	if !strings.Contains(err.Error(), "137") {
		t.Errorf("the error should explain the symptom (exit 137): %v", err)
	}
}

func TestBuildSpecCustomNeedsImage(t *testing.T) {
	m, _ := newManager(t, 32*gb)
	_, err := m.BuildSpec(SpecRequest{Name: "custom-1", TemplateID: "custom"})
	if err == nil {
		t.Fatal("the custom template with no image should fail")
	}
}

func TestCreateWithoutStartDoesNotTouchDocker(t *testing.T) {
	m, fake := newManager(t, 16*gb)
	if _, err := m.Create(context.Background(), req("smp", "4g")); err != nil {
		t.Fatalf("Create: %v", err)
	}
	for _, c := range fake.Calls {
		if strings.HasPrefix(c, "up:") || strings.HasPrefix(c, "pull:") {
			t.Errorf("creating without start must not bring anything up, calls: %v", fake.Calls)
		}
	}
	inst, err := m.Get(context.Background(), "smp")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if inst.State != instance.StateStopped {
		t.Errorf("state = %q, wanted stopped", inst.State)
	}
}

func TestCreateWithStartPullsThenUps(t *testing.T) {
	m, fake := newManager(t, 16*gb)
	r := req("smp", "4g")
	r.Start = true
	if _, err := m.Create(context.Background(), r); err != nil {
		t.Fatalf("Create: %v", err)
	}
	waitFor(t, "the instance to come up", func() bool {
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
		t.Errorf("call order = %v", order)
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
		t.Fatalf("expected ErrPortTaken, got %v", err)
	}
	if pe.Owner != "mc-a" {
		t.Errorf("the error should name who holds the port, got %q", pe.Owner)
	}
}

func TestSuggestPortSkipsTakenOnes(t *testing.T) {
	m, _ := newManager(t, 32*gb)
	if _, err := m.Create(context.Background(), req("mc-a", "4g")); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if got := m.SuggestPort(25565, "tcp"); got != 25566 {
		t.Errorf("SuggestPort = %d, wanted 25566", got)
	}
}

func TestStartRefusesWhenRAMBudgetIsExceeded(t *testing.T) {
	m, _ := newManager(t, 8*gb)

	up := req("mc-a", "4g")
	up.Start = true
	if _, err := m.Create(context.Background(), up); err != nil {
		t.Fatalf("Create: %v", err)
	}
	waitFor(t, "mc-a to come up", func() bool {
		i, _ := m.Get(context.Background(), "mc-a")
		return i.State == instance.StateRunning
	})

	if _, err := m.Create(context.Background(), req("mc-b", "4g")); err != nil {
		t.Fatalf("Create: %v", err)
	}
	err := m.Start(context.Background(), "mc-b")

	var be *ErrBudget
	if !errors.As(err, &be) {
		t.Fatalf("expected ErrBudget, got %v", err)
	}
	if be.Budget != 6*gb {
		t.Errorf("budget = %d, wanted 6 GiB", be.Budget)
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
		t.Fatalf("a stopped instance must not count against the budget: %v", err)
	}
}

func TestStopBringsInstanceDown(t *testing.T) {
	m, _ := newManager(t, 16*gb)
	r := req("smp", "4g")
	r.Start = true
	if _, err := m.Create(context.Background(), r); err != nil {
		t.Fatalf("Create: %v", err)
	}
	waitFor(t, "it to come up", func() bool {
		i, _ := m.Get(context.Background(), "smp")
		return i.State == instance.StateRunning
	})

	if err := m.Stop(context.Background(), "smp"); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	waitFor(t, "it to stop", func() bool {
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
	waitFor(t, "the failure to show up", func() bool {
		i, _ := m.Get(context.Background(), "smp")
		return i.State == instance.StateError
	})

	inst, _ := m.Get(context.Background(), "smp")
	if !strings.Contains(inst.Status, "already allocated") {
		t.Errorf("the card must show what docker said, got %q", inst.Status)
	}

	m.ClearError("smp")
	inst, _ = m.Get(context.Background(), "smp")
	if inst.State == instance.StateError {
		t.Error("ClearError did not clear the error state")
	}
}

func TestUpdateRecreatesWhenEnvChanges(t *testing.T) {
	m, fake := newManager(t, 16*gb)
	r := req("smp", "4g")
	r.Start = true
	if _, err := m.Create(context.Background(), r); err != nil {
		t.Fatalf("Create: %v", err)
	}
	waitFor(t, "it to come up", func() bool {
		i, _ := m.Get(context.Background(), "smp")
		return i.State == instance.StateRunning
	})
	before := len(fake.Calls)

	next := req("smp", "4g")
	next.Values["MAX_PLAYERS"] = "30"
	if _, err := m.Update(context.Background(), "smp", next); err != nil {
		t.Fatalf("Update: %v", err)
	}
	waitFor(t, "the recreate", func() bool {
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
		t.Errorf("an env update should bring it down and up, calls: %v", fake.Calls[before:])
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
		t.Error("identical specs should not ask for a recreate")
	}

	changed := same
	changed.Env = map[string]string{"A": "2"}
	if !NeedsRecreate(base, changed) {
		t.Error("an env change should ask for a recreate")
	}

	changed = same
	changed.MemoryLimit = "8g"
	if !NeedsRecreate(base, changed) {
		t.Error("a RAM limit change should ask for a recreate")
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
		t.Errorf("expected ErrNotFound, got %v", err)
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
		t.Errorf("state = %q, wanted archived", inst.State)
	}
	if err := m.Start(context.Background(), "smp"); err == nil {
		t.Error("an archived instance must not start before being restored")
	}
}

func TestSystemReportsCommittedMemory(t *testing.T) {
	m, _ := newManager(t, 16*gb)
	r := req("smp", "4g")
	r.Start = true
	if _, err := m.Create(context.Background(), r); err != nil {
		t.Fatalf("Create: %v", err)
	}
	waitFor(t, "it to come up", func() bool {
		i, _ := m.Get(context.Background(), "smp")
		return i.State == instance.StateRunning
	})

	info, err := m.System(context.Background())
	if err != nil {
		t.Fatalf("System: %v", err)
	}
	if info.MemoryBudget != 14*gb {
		t.Errorf("budget = %d, wanted 14 GiB", info.MemoryBudget)
	}
	if info.MemoryCommitted != 4*gb {
		t.Errorf("committed = %d, wanted 4 GiB", info.MemoryCommitted)
	}
}

func TestDeriveState(t *testing.T) {
	cases := []struct {
		name       string
		containers []dockerx.Container
		want       instance.State
	}{
		{"no container", nil, instance.StateStopped},
		{"running", []dockerx.Container{{State: "running"}}, instance.StateRunning},
		{"healthcheck still starting", []dockerx.Container{{State: "running", Health: "starting"}}, instance.StateStarting},
		{"unhealthy", []dockerx.Container{{State: "running", Health: "unhealthy"}}, instance.StateError},
		{"exited clean", []dockerx.Container{{State: "exited", ExitCode: 0}}, instance.StateStopped},
		{"OOM killed", []dockerx.Container{{State: "exited", ExitCode: 137}}, instance.StateError},
		{"created but not started", []dockerx.Container{{State: "created"}}, instance.StateProvisioning},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, _, _, _ := deriveState(instance.Spec{}, tc.containers, nil)
			if got != tc.want {
				t.Errorf("deriveState = %q, wanted %q", got, tc.want)
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
			t.Errorf("event = %+v", ev)
		}
	case <-time.After(time.Second):
		t.Fatal("the event never arrived")
	}

	cancel()
	h.Publish(Event{Type: "instance.changed", Instance: "smp"})
	if _, open := <-ch; open {
		t.Error("the channel should be closed after cancel")
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
		t.Fatal("Publish blocked on a slow subscriber")
	}
}

func TestUpdateImageRecreatesOnlyWhenTheImageChanged(t *testing.T) {
	m, fake := newManager(t, 16*gb)
	r := req("smp", "4g")
	r.Start = true
	if _, err := m.Create(context.Background(), r); err != nil {
		t.Fatalf("Create: %v", err)
	}
	waitFor(t, "it to come up", func() bool {
		i, _ := m.Get(context.Background(), "smp")
		return i.State == instance.StateRunning
	})

	image := "itzg/minecraft-server:java21"
	fake.ImageIDs[image] = "sha256:old"
	fake.PulledImageIDs[image] = "sha256:old"
	before := len(fake.Calls)

	if err := m.UpdateImage(context.Background(), "smp"); err != nil {
		t.Fatalf("UpdateImage: %v", err)
	}
	waitFor(t, "the check to finish", func() bool {
		i, _ := m.Get(context.Background(), "smp")
		return i.Operation == nil
	})

	for _, c := range fake.Calls[before:] {
		if strings.HasPrefix(c, "up:") {
			t.Errorf("recreated without a new image, calls: %v", fake.Calls[before:])
		}
	}

	fake.PulledImageIDs[image] = "sha256:new"
	before = len(fake.Calls)
	if err := m.UpdateImage(context.Background(), "smp"); err != nil {
		t.Fatalf("UpdateImage: %v", err)
	}
	waitFor(t, "the recreate", func() bool {
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
		t.Errorf("a new image should recreate the container, calls: %v", fake.Calls[before:])
	}
}

func TestUpdateImagePublishesWhatHappened(t *testing.T) {
	m, fake := newManager(t, 16*gb)
	if _, err := m.Create(context.Background(), req("smp", "4g")); err != nil {
		t.Fatalf("Create: %v", err)
	}
	image := "itzg/minecraft-server:java21"
	fake.ImageIDs[image] = "sha256:old"
	fake.PulledImageIDs[image] = "sha256:old"

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
				if !strings.Contains(ev.Message, "newest image") {
					t.Errorf("unclear message: %q", ev.Message)
				}
				return
			}
		case <-deadline:
			t.Fatal("no event saying it was already up to date")
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
		t.Error("an archived instance must not be updated before being restored")
	}
}

func hostContainer(name, project string) dockerx.HostContainer {
	return dockerx.HostContainer{
		Name:    name,
		Image:   "jellyfin/jellyfin:latest",
		State:   "running",
		Status:  "Up 35 hours",
		Project: project,
		Service: name,
		WorkDir: "/home/vitorcds/servidor/" + project,
		Ports:   []dockerx.HostPort{{Host: 8096, Container: 8096, Protocol: "tcp"}},
	}
}

func TestListShowsAContainerThatWasAlreadyThere(t *testing.T) {
	m, fake := newManager(t, 16*gb)
	fake.HostList = []dockerx.HostContainer{hostContainer("jellyfin", "media")}

	list, err := m.List(t.Context())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 instance, got %d", len(list))
	}

	got := list[0]
	if !got.External || got.Name != "jellyfin" || got.Project != "media" {
		t.Errorf("the external container came out wrong: %+v", got)
	}
	if got.State != instance.StateRunning {
		t.Errorf("state = %q", got.State)
	}
	if len(got.Ports) != 1 || got.Ports[0].Host != 8096 {
		t.Errorf("ports = %+v", got.Ports)
	}
}

func TestListCarriesTheNetworksOfEveryContainer(t *testing.T) {
	m, fake := newManager(t, 16*gb)
	if _, err := m.Create(t.Context(), SpecRequest{Name: "smp", TemplateID: "minecraft-java"}); err != nil {
		t.Fatal(err)
	}

	outside := hostContainer("nextcloud-db", "nextcloud")
	outside.Networks = []string{"nextcloud_default"}
	fake.HostList = []dockerx.HostContainer{
		{Name: "smp", State: "running", Status: "Up 1 minute", WorkDir: m.store.Dir("smp"), Networks: []string{"smp_default"}},
		outside,
	}

	list, err := m.List(t.Context())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	byName := map[string][]string{}
	for _, inst := range list {
		byName[inst.Name] = inst.Networks
	}
	if got := byName["smp"]; len(got) != 1 || got[0] != "smp_default" {
		t.Errorf("networks of the panel instance = %v", got)
	}
	if got := byName["nextcloud-db"]; len(got) != 1 || got[0] != "nextcloud_default" {
		t.Errorf("networks of the outside container = %v", got)
	}
}

func TestListDoesNotDuplicateAPanelInstance(t *testing.T) {
	m, fake := newManager(t, 16*gb)
	inst, err := m.Create(t.Context(), SpecRequest{Name: "smp", TemplateID: "minecraft-java"})
	if err != nil {
		t.Fatal(err)
	}

	fake.HostList = []dockerx.HostContainer{{
		Name: "smp", Image: inst.Image, State: "running", Status: "Up 1 minute",
		Project: "smp", Service: "smp", WorkDir: m.store.Dir("smp"),
	}}

	list, err := m.List(t.Context())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("the panel instance was listed twice: %+v", list)
	}
	if list[0].External {
		t.Error("an instance created by the panel is not external")
	}
}

func TestListHidesThePanelItself(t *testing.T) {
	m, fake := newManager(t, 16*gb)
	self, err := os.Hostname()
	if err != nil {
		t.Fatal(err)
	}
	fake.HostList = []dockerx.HostContainer{
		{ID: self + "abc123", Name: "okdock", Image: "okdock:local", State: "running"},
		hostContainer("jellyfin", "media"),
	}

	list, err := m.List(t.Context())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	for _, inst := range list {
		if inst.Name == "okdock" {
			t.Fatal("the panel must not show up on its own board with a stop button")
		}
	}
	if len(list) != 1 {
		t.Errorf("expected only jellyfin: %+v", list)
	}
}

func TestActionsOnAnExternalContainer(t *testing.T) {
	m, fake := newManager(t, 16*gb)
	fake.HostList = []dockerx.HostContainer{hostContainer("jellyfin", "media")}
	ctx := t.Context()

	actions := []struct {
		name string
		run  func(context.Context, string) error
		call string
	}{
		{"Stop", m.Stop, "container-stop:jellyfin"},
		{"Start", m.Start, "container-start:jellyfin"},
		{"Restart", m.Restart, "container-restart:jellyfin"},
	}
	for _, a := range actions {
		if err := a.run(ctx, "jellyfin"); err != nil {
			t.Fatalf("%s: %v", a.name, err)
		}
		// the operation leaves the map when the goroutine ends, only then Calls is stable
		waitFor(t, "the "+a.name+" operation to finish", func() bool { return m.operation("jellyfin") == nil })
		if !containsCall(fake.Calls, a.call) {
			t.Errorf("missing call %q: %v", a.call, fake.Calls)
		}
	}
}

func TestAFailureOnAnExternalContainerBecomesAnErrorOnTheBoard(t *testing.T) {
	m, fake := newManager(t, 16*gb)
	fake.HostList = []dockerx.HostContainer{hostContainer("jellyfin", "media")}
	fake.FailAction = map[string]error{"jellyfin": errors.New("permission denied")}
	ctx := t.Context()

	if err := m.Stop(ctx, "jellyfin"); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	waitFor(t, "the docker error to reach the operation", func() bool {
		op := m.operation("jellyfin")
		return op != nil && op.Error != ""
	})

	list, err := m.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if list[0].State != instance.StateError {
		t.Errorf("state = %q, a docker refusal must not pass in silence", list[0].State)
	}
	if !strings.Contains(list[0].Status, "permission denied") {
		t.Errorf("status = %q, wanted what docker said", list[0].Status)
	}

	m.ClearError("jellyfin")
	if m.operation("jellyfin") != nil {
		t.Error("clearing the error must unblock the external container")
	}
}

func TestEditingAnExternalContainerIsRefused(t *testing.T) {
	m, fake := newManager(t, 16*gb)
	fake.HostList = []dockerx.HostContainer{hostContainer("jellyfin", "media")}
	ctx := t.Context()

	if _, err := m.Update(ctx, "jellyfin", SpecRequest{Name: "jellyfin"}); !errors.Is(err, ErrExternal) {
		t.Errorf("Update = %v, wanted ErrExternal", err)
	}
	if err := m.Delete(ctx, "jellyfin", true); !errors.Is(err, ErrExternal) {
		t.Errorf("Delete = %v, wanted ErrExternal", err)
	}
	if err := m.SetArchived(ctx, "jellyfin", true); !errors.Is(err, ErrExternal) {
		t.Errorf("SetArchived = %v, wanted ErrExternal", err)
	}
	if err := m.UpdateImage(ctx, "jellyfin"); !errors.Is(err, ErrExternal) {
		t.Errorf("UpdateImage = %v, wanted ErrExternal", err)
	}
}

func containsCall(calls []string, want string) bool {
	for _, c := range calls {
		if c == want {
			return true
		}
	}
	return false
}

func TestAnExternalContainerNeverHasANullList(t *testing.T) {
	m, fake := newManager(t, 16*gb)
	noPorts := hostContainer("duckdns", "duckdns")
	noPorts.Ports = nil
	fake.HostList = []dockerx.HostContainer{noPorts}

	list, err := m.List(t.Context())
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	raw, err := json.Marshal(list[0])
	if err != nil {
		t.Fatal(err)
	}
	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"ports", "mounts"} {
		if body[field] == nil {
			t.Errorf("%s came as null, the screen counts the items without checking: %s", field, raw)
		}
	}
}

func TestAnExternalContainerLandsInTheImageCategory(t *testing.T) {
	m, fake := newManager(t, 16*gb)
	jelly := hostContainer("jellyfin", "media")
	db := hostContainer("nextcloud-mysql", "nextcloud")
	db.Image = "mariadb:10.6"
	fake.HostList = []dockerx.HostContainer{jelly, db}

	list, err := m.List(t.Context())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	byName := map[string]string{}
	for _, inst := range list {
		byName[inst.Name] = inst.Category
	}
	if byName["jellyfin"] != "media" {
		t.Errorf("jellyfin landed in %q, with no category the board dumps everything in Other", byName["jellyfin"])
	}
	if byName["nextcloud-mysql"] != "database" {
		t.Errorf("mariadb landed in %q", byName["nextcloud-mysql"])
	}
}

func TestAnExternalContainerUsesLabelAndPortWhenTheNameSaysNothing(t *testing.T) {
	m, fake := newManager(t, 16*gb)
	labeled := hostContainer("internal", "homemade")
	labeled.Image = "registry.local/internal:1"
	labeled.Labels = map[string]string{
		"org.opencontainers.image.source": "https://github.com/jellyfin/jellyfin",
	}
	labeled.Ports = nil

	ported := hostContainer("server", "homemade")
	ported.Image = "registry.local/server:1"
	ported.Ports = []dockerx.HostPort{{Host: 30000, Container: 25565, Protocol: "tcp"}}

	fake.HostList = []dockerx.HostContainer{labeled, ported}

	list, err := m.List(t.Context())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	byName := map[string]string{}
	for _, inst := range list {
		byName[inst.Name] = inst.Category
	}
	if byName["internal"] != "media" {
		t.Errorf("the image label should win: category = %q", byName["internal"])
	}
	if byName["server"] != "games" {
		t.Errorf("port 25565 should win: category = %q", byName["server"])
	}
}

// a stack the panel did not write, with a second service that must survive
const outsideCompose = `name: nextcloud
services:
  app:
    image: nextcloud:apache
    container_name: nextcloud
    restart: unless-stopped
    ports:
      - "8080:80"
    environment:
      MYSQL_HOST: nextcloud-mysql
      TRUSTED_PROXIES: 10.0.0.0/8
    volumes:
      - /srv/nextcloud/html:/var/www/html
    deploy:
      resources:
        limits:
          memory: 2G
  cron:
    image: nextcloud:apache
    container_name: nextcloud-cron
    entrypoint: /cron.sh
`

func outsideStack(t *testing.T) (dir string, container dockerx.HostContainer) {
	t.Helper()
	dir = t.TempDir()
	file := filepath.Join(dir, "docker-compose.yml")
	if err := os.WriteFile(file, []byte(outsideCompose), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir, dockerx.HostContainer{
		Name:    "nextcloud",
		Image:   "nextcloud:apache",
		State:   "running",
		Status:  "Up 22 hours",
		Project: "nextcloud",
		Service: "app",
		WorkDir: dir,
		Labels:  map[string]string{"com.docker.compose.project.config_files": file},
		Ports:   []dockerx.HostPort{{Host: 8080, Container: 80, Protocol: "tcp"}},
	}
}

func TestListReadsTheComposeOfAnOutsideContainer(t *testing.T) {
	m, fake := newManager(t, 16*gb)
	_, c := outsideStack(t)
	fake.HostList = []dockerx.HostContainer{c}

	list, err := m.List(t.Context())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	got := list[0]
	if !got.External || !got.Editable {
		t.Fatalf("the container is not editable: %+v", got)
	}
	// none of this is in docker ps, it only comes from the file
	if got.MemoryLimit != "2G" || got.Restart != "unless-stopped" {
		t.Errorf("limits = %q %q", got.MemoryLimit, got.Restart)
	}
	if got.Env["MYSQL_HOST"] != "nextcloud-mysql" {
		t.Errorf("env = %v", got.Env)
	}
	if len(got.Mounts) != 1 || got.Mounts[0].Container != "/var/www/html" {
		t.Errorf("mounts = %+v", got.Mounts)
	}
}

func TestListWillNotEditAComposeItCannotWriteBack(t *testing.T) {
	m, fake := newManager(t, 16*gb)
	dir, c := outsideStack(t)
	file := filepath.Join(dir, "docker-compose.yml")
	if err := os.WriteFile(file, []byte("include:\n  - other.yml\n"+outsideCompose), 0o644); err != nil {
		t.Fatal(err)
	}
	fake.HostList = []dockerx.HostContainer{c}

	list, err := m.List(t.Context())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if list[0].Editable {
		t.Error("a file with an include cannot be written back")
	}
}

func TestUpdateEditsOneServiceOfAnOutsideStack(t *testing.T) {
	m, fake := newManager(t, 16*gb)
	dir, c := outsideStack(t)
	fake.HostList = []dockerx.HostContainer{c}
	file := filepath.Join(dir, "docker-compose.yml")

	spec, err := m.Update(t.Context(), "nextcloud", SpecRequest{
		Image:       "nextcloud:30-apache",
		Values:      map[string]string{"MYSQL_HOST": "db"},
		Ports:       []instance.PortBinding{{Host: 8081, Container: 80, Protocol: "tcp"}},
		MemoryLimit: "3g",
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if spec.Image != "nextcloud:30-apache" {
		t.Errorf("spec = %+v", spec)
	}

	raw, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, want := range []string{
		"image: nextcloud:30-apache",
		`"8081:80/tcp"`,
		"MYSQL_HOST: db",
		"container_name: nextcloud-cron",
		"entrypoint: /cron.sh",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("%q is not in the file:\n%s", want, text)
		}
	}
	// a variable the form never showed keeps the value the owner wrote
	if !strings.Contains(text, "TRUSTED_PROXIES: 10.0.0.0/8") {
		t.Errorf("an unknown variable was dropped:\n%s", text)
	}

	waitFor(t, "the operation to finish", func() bool { return m.operation("nextcloud") == nil })
	if !slices.Contains(fake.Calls, "up:"+dir+"#app") {
		t.Errorf("only the edited service goes back up, calls = %v", fake.Calls)
	}
}

func TestUpdateImageOfAnOutsideContainer(t *testing.T) {
	m, fake := newManager(t, 16*gb)
	dir, c := outsideStack(t)
	fake.HostList = []dockerx.HostContainer{c}
	fake.ImageIDs["nextcloud:apache"] = "sha256:old"
	fake.PulledImageIDs = map[string]string{"nextcloud:apache": "sha256:new"}

	if err := m.UpdateImage(t.Context(), "nextcloud"); err != nil {
		t.Fatalf("UpdateImage: %v", err)
	}
	waitFor(t, "the operation to finish", func() bool { return m.operation("nextcloud") == nil })

	for _, want := range []string{"pull:" + dir + "#app", "up:" + dir + "#app"} {
		if !slices.Contains(fake.Calls, want) {
			t.Errorf("%q is not among the calls: %v", want, fake.Calls)
		}
	}
}

func TestUpdateRefusesAContainerWithNoComposeFile(t *testing.T) {
	m, fake := newManager(t, 16*gb)
	fake.HostList = []dockerx.HostContainer{hostContainer("jellyfin", "media")}

	if _, err := m.Update(t.Context(), "jellyfin", SpecRequest{Image: "jellyfin:next"}); !errors.Is(err, ErrExternal) {
		t.Fatalf("err = %v, want the external error", err)
	}
	if err := m.UpdateImage(t.Context(), "jellyfin"); !errors.Is(err, ErrExternal) {
		t.Fatalf("err = %v, want the external error", err)
	}
}

func TestAComposeFileThePanelCannotSeeSaysSo(t *testing.T) {
	m, fake := newManager(t, 16*gb)
	_, c := outsideStack(t)
	// the path the daemon reports is a host path, and the panel container does not have it mounted
	c.Labels = map[string]string{"com.docker.compose.project.config_files": "/home/somebody/stack/docker-compose.yml"}
	fake.HostList = []dockerx.HostContainer{c}

	list, err := m.List(t.Context())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	got := list[0]
	if got.Editable {
		t.Fatal("a file it cannot open is not editable")
	}
	if got.ReadOnly != "not_visible" {
		t.Errorf("readOnly = %q, want not_visible", got.ReadOnly)
	}
	if got.ComposeFile != "/home/somebody/stack/docker-compose.yml" {
		t.Errorf("composeFile = %q, the screen needs the path to name it", got.ComposeFile)
	}
	if err := m.UpdateImage(t.Context(), "nextcloud"); !errors.Is(err, ErrExternal) {
		t.Errorf("err = %v, want the external error", err)
	}
}

func TestSuggestFromTheImageItself(t *testing.T) {
	m, fake := newManager(t, 16*gb)
	fake.ImageConfigs["jellyfin/jellyfin:10.9"] = dockerx.ImageInfo{
		Ports:   []dockerx.HostPort{{Container: 8096, Protocol: "tcp"}},
		Volumes: []string{"/cache", "/config"},
	}

	got, err := m.SuggestFromImage(t.Context(), "jellyfin/jellyfin:10.9")
	if err != nil {
		t.Fatalf("SuggestFromImage: %v", err)
	}
	if len(got.Ports) != 1 || got.Ports[0].Container != 8096 || got.Ports[0].DefaultHost != 8096 {
		t.Fatalf("ports = %+v", got.Ports)
	}
	want := []template.Volume{{Container: "/cache"}, {Container: "/config"}}
	for i := range want {
		if got.Volumes[i] != want[i] {
			t.Fatalf("volumes = %+v, want %+v", got.Volumes, want)
		}
	}
}

func TestSuggestFallsBackToAContainerForTheVolumes(t *testing.T) {
	m, fake := newManager(t, 16*gb)
	// the whole linuxserver family declares no VOLUME, only the container knows
	fake.ImageConfigs["lscr.io/linuxserver/sonarr"] = dockerx.ImageInfo{}
	fake.MountsByImage["lscr.io/linuxserver/sonarr"] = []string{"/config", "/downloads", "/media"}

	got, err := m.SuggestFromImage(t.Context(), "lscr.io/linuxserver/sonarr")
	if err != nil {
		t.Fatalf("SuggestFromImage: %v", err)
	}
	if len(got.Volumes) != 3 || got.Volumes[0].Container != "/config" {
		t.Fatalf("volumes = %+v", got.Volumes)
	}
	if len(got.Ports) != 0 {
		t.Fatalf("ports = %+v, the image exposes none", got.Ports)
	}
}

func TestSuggestDodgesAPortAlreadyTaken(t *testing.T) {
	m, fake := newManager(t, 16*gb)
	if _, err := m.Create(t.Context(), SpecRequest{Name: "smp", TemplateID: "minecraft-java"}); err != nil {
		t.Fatal(err)
	}
	fake.ImageConfigs["itzg/minecraft-server"] = dockerx.ImageInfo{
		Ports: []dockerx.HostPort{{Container: 25565, Protocol: "tcp"}},
	}

	got, err := m.SuggestFromImage(t.Context(), "itzg/minecraft-server")
	if err != nil {
		t.Fatalf("SuggestFromImage: %v", err)
	}
	if got.Ports[0].DefaultHost == 25565 {
		t.Fatalf("the suggested host port collides with the instance already there: %+v", got.Ports)
	}
}

func TestSuggestForAnImageNobodyHasPulled(t *testing.T) {
	m, _ := newManager(t, 16*gb)

	got, err := m.SuggestFromImage(t.Context(), "somebody/never-pulled")
	if err != nil {
		t.Fatalf("SuggestFromImage: %v", err)
	}
	// no image, no container and no registry: nothing to suggest is an empty answer, not a failure
	if len(got.Ports) != 0 || len(got.Volumes) != 0 {
		t.Fatalf("got = %+v", got)
	}
}

func TestMountsForNamesAFolderAfterThePathInside(t *testing.T) {
	got := mountsFor([]template.Volume{
		{Container: "/config"},
		{Container: "/media/tv"},
		{Container: "/downloads/"},
	})
	want := []instance.Mount{
		{Host: "./config", Container: "/config"},
		{Host: "./tv", Container: "/media/tv"},
		{Host: "./downloads", Container: "/downloads/"},
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("mounts = %+v, want %+v", got, want)
		}
	}
}

func TestMountsForKeepsTwoVolumesOutOfTheSameFolder(t *testing.T) {
	got := mountsFor([]template.Volume{{Container: "/config"}, {Container: "/etc/config"}})
	if got[0].Host != "./config" || got[1].Host != "./etc-config" {
		t.Fatalf("mounts = %+v, the second one took the folder of the first", got)
	}
}
