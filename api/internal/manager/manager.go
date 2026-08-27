package manager

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/VitorCdSouza/okdock/api/internal/compose"
	"github.com/VitorCdSouza/okdock/api/internal/dockerx"
	"github.com/VitorCdSouza/okdock/api/internal/duckdns"
	"github.com/VitorCdSouza/okdock/api/internal/hostfs"
	"github.com/VitorCdSouza/okdock/api/internal/instance"
	"github.com/VitorCdSouza/okdock/api/internal/registry"
	"github.com/VitorCdSouza/okdock/api/internal/store"
	"github.com/VitorCdSouza/okdock/api/internal/system"
	"github.com/VitorCdSouza/okdock/api/internal/template"
)

type Options struct {
	Store         *store.Store
	Templates     *template.Catalog
	Docker        dockerx.Runner
	System        system.Reader
	DNS           duckdns.Client
	Registry      registry.Client
	MemoryReserve int64
	Now           func() time.Time
}

const DefaultMemoryReserve = 2 << 30

type Manager struct {
	store     *store.Store
	templates *template.Catalog
	docker    dockerx.Runner
	sys       system.Reader
	reserve   int64
	now       func() time.Time

	mu  sync.Mutex
	ops map[string]*instance.Operation

	dns      duckdns.Client
	registry registry.Client

	dnsMu  sync.Mutex
	dnsCfg store.DNSConfig
	dnsBg  sync.WaitGroup

	hub *Hub
}

func New(o Options) *Manager {
	reserve := o.MemoryReserve
	if reserve == 0 {
		reserve = DefaultMemoryReserve
	}
	now := o.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	dnsCfg, err := o.Store.LoadDNS()
	if err != nil {
		slog.Warn("could not read the DNS config", "file", o.Store.DNSPath(), "err", err)
	}

	return &Manager{
		store:     o.Store,
		templates: o.Templates,
		docker:    o.Docker,
		sys:       o.System,
		reserve:   reserve,
		now:       now,
		ops:       map[string]*instance.Operation{},
		dns:       o.DNS,
		registry:  o.Registry,
		dnsCfg:    dnsCfg,
		hub:       NewHub(),
	}
}

func (m *Manager) Events() *Hub { return m.hub }

func (m *Manager) List(ctx context.Context) ([]instance.Instance, error) {
	specs, err := m.store.List()
	if err != nil {
		return nil, err
	}
	containers, err := m.docker.PSAll(ctx)
	if err != nil {
		slog.Debug("could not list the host containers", "err", err)
	}
	specs = dropSelf(specs, m.store.Root(), containers)

	out := make([]instance.Instance, 0, len(specs))
	running := make([]string, 0, len(specs))
	for _, spec := range specs {
		inst := m.hydrate(ctx, spec)
		if inst.State == instance.StateRunning || inst.State == instance.StateStarting {
			running = append(running, spec.Name)
		}
		out = append(out, inst)
	}
	attachNetworks(out, containers)

	external, upExternal := m.listExternal(out, containers)
	out = append(out, external...)
	running = append(running, upExternal...)

	m.attachStats(ctx, out, running)
	return out, nil
}

// the panel folder can sit inside the instance folder, and the panel is no instance of itself
func dropSelf(specs []instance.Spec, root string, containers []dockerx.HostContainer) []instance.Spec {
	self, _ := os.Hostname()
	if self == "" || root == "" {
		return specs
	}
	dir := ""
	for _, c := range containers {
		if strings.HasPrefix(c.ID, self) {
			dir = filepath.Clean(c.WorkDir)
			break
		}
	}
	if dir == "" || filepath.Dir(dir) != filepath.Clean(root) {
		return specs
	}
	name := filepath.Base(dir)
	out := make([]instance.Spec, 0, len(specs))
	for _, spec := range specs {
		if spec.Name != name {
			out = append(out, spec)
		}
	}
	return out
}

// the managed container carries the instance name, which is how docker ps finds it again
func attachNetworks(list []instance.Instance, containers []dockerx.HostContainer) {
	byName := make(map[string][]string, len(containers))
	for _, c := range containers {
		byName[c.Name] = c.Networks
	}
	for i := range list {
		list[i].Networks = byName[list[i].Name]
	}
}

func (m *Manager) listExternal(managed []instance.Instance, containers []dockerx.HostContainer) (list []instance.Instance, running []string) {
	// inside the container the hostname is the container own short id
	self, _ := os.Hostname()

	ours := make(map[string]bool, len(managed)*2)
	for _, inst := range managed {
		ours[filepath.Clean(inst.Dir)] = true
		ours["name:"+inst.Name] = true
	}

	now := m.now()
	for _, c := range containers {
		if self != "" && strings.HasPrefix(c.ID, self) {
			continue
		}
		if c.WorkDir != "" && ours[filepath.Clean(c.WorkDir)] {
			continue
		}
		if ours["name:"+c.Name] {
			continue
		}

		inst := instance.Instance{
			Spec: instance.Spec{
				Name:      c.Name,
				Image:     c.Image,
				Category:  string(m.templates.CategoryFor(hintsFor(c))),
				Env:       map[string]string{},
				Ports:     []instance.PortBinding{},
				Mounts:    []instance.Mount{},
				CreatedAt: now,
				UpdatedAt: now,
			},
			Dir:      c.WorkDir,
			Networks: c.Networks,
			External: true,
			Project:  c.Project,
			Service:  c.Service,
			Status:   c.Status,
			Health:   c.Health,
		}
		for _, p := range c.Ports {
			inst.Ports = append(inst.Ports, instance.PortBinding{
				Host: p.Host, Container: p.Container, Protocol: p.Protocol,
			})
		}
		readExternalCompose(&inst, c)
		inst.Operation = m.operation(c.Name)
		inst.State = externalState(c)
		if inst.State == instance.StateError && c.ExitCode != 0 {
			code := c.ExitCode
			inst.ExitCode = &code
		}
		if op := inst.Operation; op != nil {
			switch {
			case op.Error != "":
				inst.State, inst.Status = instance.StateError, op.Error
			case op.Kind == OpStart:
				inst.State = instance.StateStarting
			}
		}
		if isUp(inst.State) {
			running = append(running, c.Name)
		}
		list = append(list, inst)
	}

	sort.Slice(list, func(i, j int) bool { return list[i].Name < list[j].Name })
	return list, running
}

// the compose file of an outside container is the whole config, and what it cannot express locks it
func readExternalCompose(inst *instance.Instance, c dockerx.HostContainer) {
	file := composeFileOf(c)
	if file == "" || c.Service == "" {
		inst.ReadOnly = "no_compose"
		return
	}
	inst.ComposeFile = file

	raw, err := os.ReadFile(file)
	if err != nil {
		// the panel runs in a container, so a path the daemon reported is not always one it can open
		inst.ReadOnly = "not_visible"
		slog.Warn("the compose file of an outside container is not visible from the panel",
			"container", c.Name, "file", file, "err", err)
		return
	}
	project, err := compose.Parse(raw)
	if err != nil {
		inst.ReadOnly = "unreadable"
		slog.Warn("could not parse the compose file of an outside container",
			"container", c.Name, "file", file, "err", err)
		return
	}
	svc, ok := project.Service(c.Service)
	if !ok {
		inst.ReadOnly = "no_compose"
		return
	}

	spec := svc.Spec()
	spec.Name = c.Name
	spec.CreatedAt, spec.UpdatedAt = inst.CreatedAt, inst.UpdatedAt
	if spec.Category == "" {
		spec.Category = inst.Category
	}
	if len(spec.Ports) == 0 {
		spec.Ports = inst.Ports
	}
	inst.Spec = spec
	inst.ComposeFile = file
	inst.Editable = len(project.Unsupported) == 0
	if !inst.Editable {
		inst.ReadOnly = "unsupported"
		slog.Warn("the compose file of an outside container has what the panel does not write back",
			"container", c.Name, "file", file, "what", strings.Join(project.Unsupported, ", "))
	}
}

// docker records the files it read, and more than one means the panel reads none of them
func composeFileOf(c dockerx.HostContainer) string {
	files := strings.Split(c.Labels["com.docker.compose.project.config_files"], ",")
	if len(files) != 1 {
		return ""
	}
	file := strings.TrimSpace(files[0])
	if file == "" {
		return ""
	}
	if !filepath.IsAbs(file) {
		file = filepath.Join(c.WorkDir, file)
	}
	return file
}

func externalState(c dockerx.HostContainer) instance.State {
	switch c.State {
	case "running":
		switch c.Health {
		case "starting":
			return instance.StateStarting
		case "unhealthy":
			return instance.StateError
		default:
			return instance.StateRunning
		}
	case "restarting":
		return instance.StateStarting
	case "created":
		return instance.StateProvisioning
	case "exited", "dead":
		if c.ExitCode != 0 {
			return instance.StateError
		}
		return instance.StateStopped
	default:
		return instance.StateStopped
	}
}

func hintsFor(c dockerx.HostContainer) template.Hints {
	ports := make([]int, 0, len(c.Ports))
	for _, p := range c.Ports {
		ports = append(ports, p.Container)
	}
	return template.Hints{
		Image:   c.Image,
		Name:    c.Name,
		Service: c.Service,
		Labels:  c.Labels,
		Ports:   ports,
	}
}

func (m *Manager) external(ctx context.Context, name string) (instance.Instance, bool) {
	list, err := m.List(ctx)
	if err != nil {
		return instance.Instance{}, false
	}
	for _, inst := range list {
		if inst.External && inst.Name == name {
			return inst, true
		}
	}
	return instance.Instance{}, false
}

func (m *Manager) Get(ctx context.Context, name string) (instance.Instance, error) {
	spec, err := m.store.Get(name)
	if err != nil {
		if inst, ok := m.external(ctx, name); ok {
			return inst, nil
		}
		return instance.Instance{}, err
	}
	list := []instance.Instance{m.hydrate(ctx, spec)}
	if isUp(list[0].State) {
		m.attachStats(ctx, list, []string{name})
	}
	return list[0], nil
}

func (m *Manager) hydrate(ctx context.Context, spec instance.Spec) instance.Instance {
	inst := instance.Instance{Spec: spec, Dir: m.store.Dir(spec.Name), Editable: true}
	inst.ComposeFile = m.store.ComposePath(spec.Name)
	inst.Operation = m.operation(spec.Name)
	inst.DNS = m.dnsFor(spec.Name)

	// a compose file nobody can parse still has a folder and containers, so it goes to the board
	if spec.Unreadable != "" {
		inst.State = instance.StateError
		inst.Status = spec.Unreadable
		inst.Editable = false
		inst.ReadOnly = "unreadable"
		return inst
	}

	containers, err := m.docker.PS(ctx, inst.Dir)
	if err != nil {
		inst.State = instance.StateError
		inst.Status = err.Error()
		return inst
	}
	inst.State, inst.Status, inst.Health, inst.ExitCode = deriveState(spec, containers, inst.Operation)
	return inst
}

func deriveState(spec instance.Spec, containers []dockerx.Container, op *instance.Operation) (instance.State, string, string, *int) {
	if op != nil {
		if op.Error != "" {
			return instance.StateError, op.Error, "", nil
		}
		switch op.Kind {
		case OpProvision:
			return instance.StateProvisioning, op.Message, "", nil
		case OpUpdate:
			return instance.StateUpdating, op.Message, "", nil
		case OpStart:
			return instance.StateStarting, op.Message, "", nil
		}
	}
	if spec.Archived {
		return instance.StateArchived, "", "", nil
	}
	if len(containers) == 0 {
		return instance.StateStopped, "", "", nil
	}
	c := containers[0]
	switch c.State {
	case "running":
		switch c.Health {
		case "starting":
			return instance.StateStarting, c.Status, c.Health, nil
		case "unhealthy":
			return instance.StateError, c.Status, c.Health, nil
		default:
			return instance.StateRunning, c.Status, c.Health, nil
		}
	case "restarting":
		return instance.StateStarting, c.Status, c.Health, nil
	case "created":
		return instance.StateProvisioning, c.Status, c.Health, nil
	case "paused":
		return instance.StateStopped, c.Status, c.Health, nil
	case "exited", "dead":
		code := c.ExitCode
		if code != 0 {
			return instance.StateError, c.Status, c.Health, &code
		}
		return instance.StateStopped, c.Status, c.Health, &code
	default:
		return instance.StateStopped, c.Status, c.Health, nil
	}
}

func (m *Manager) attachStats(ctx context.Context, list []instance.Instance, names []string) {
	if len(names) == 0 {
		return
	}
	stats, err := m.docker.Stats(ctx, names)
	if err != nil || len(stats) == 0 {
		return
	}
	byName := make(map[string]dockerx.Stats, len(stats))
	for _, s := range stats {
		byName[s.Name] = s
	}
	for i := range list {
		if s, ok := byName[list[i].Name]; ok {
			list[i].Stats = &instance.Stats{
				CPUPercent:  s.CPUPercent,
				MemoryBytes: s.MemoryBytes,
				MemoryLimit: s.MemoryLimit,
			}
		}
	}
}

func (m *Manager) System(ctx context.Context) (SystemInfo, error) {
	info, err := m.sys.Read(m.diskPath())
	if err != nil {
		return SystemInfo{}, err
	}
	out := SystemInfo{
		Info:          info,
		MemoryReserve: m.reserve,
		Root:          m.store.Root(),
		TemplatesRoot: m.store.TemplatesDir(),
	}
	out.MemoryBudget = info.MemoryTotal - m.reserve

	if v, err := m.docker.Version(ctx); err == nil {
		out.DockerVersion = v
	} else {
		out.DockerError = err.Error()
	}

	list, err := m.List(ctx)
	if err == nil {
		for _, i := range list {
			n, _ := instance.ParseMemory(i.MemoryLimit)
			out.MemoryPlanned += n
			if isUp(i.State) {
				out.MemoryCommitted += n
			}
		}
		out.InstanceCount = len(list)
	}
	return out, nil
}

// with no instance folder chosen the numbers of the screen still come from somewhere
func (m *Manager) diskPath() string {
	if root := m.store.Root(); root != "" {
		return root
	}
	return "/"
}

// a folder the panel cannot open is one docker would resolve to somewhere else entirely
func (m *Manager) SetRoot(root string) error {
	clean := filepath.Clean(root)
	if clean == m.store.ConfigRoot || clean == m.store.DefaultTemplatesDir() {
		return &store.InvalidRootError{Reason: "panel_folder", Path: root}
	}
	// a relative path is the store own refusal, and it says which field is wrong
	if filepath.IsAbs(root) {
		if err := m.browser().Reachable(root); err != nil {
			return err
		}
	}
	if err := m.store.SetRoot(root); err != nil {
		return err
	}
	m.hub.Publish(Event{Type: "instance.changed"})
	return nil
}

// moving the folder does not move what is in it, the catalog just answers from the new one
// BrowseDirs and MakeDir answer the folder picker, inside the root and whatever else is bind mounted here
func (m *Manager) BrowseDirs(dir string) (hostfs.Listing, error) {
	return m.browser().List(dir)
}

func (m *Manager) MakeDir(dir string) (string, error) {
	return m.browser().Mkdir(dir)
}

func (m *Manager) browser() *hostfs.Browser {
	return hostfs.New(func() []string {
		// the panel own folders read one way in here and another out there, no instance goes in them
		ours := []string{m.store.ConfigRoot, m.store.DefaultTemplatesDir()}
		var roots []string
		if root := m.store.Root(); root != "" {
			roots = append(roots, root)
		}
		for _, mount := range hostfs.BindMounts() {
			if !slices.Contains(ours, mount) {
				roots = append(roots, mount)
			}
		}
		return roots
	})
}

func (m *Manager) SetTemplatesDir(dir string) error {
	if filepath.IsAbs(dir) {
		if err := m.browser().Reachable(dir); err != nil {
			return err
		}
	}
	if err := m.store.SetTemplatesDir(dir); err != nil {
		return err
	}
	if err := m.templates.SetDir(m.store.TemplatesDir()); err != nil {
		return err
	}
	m.hub.Publish(Event{Type: "instance.changed"})
	return nil
}

type SystemInfo struct {
	system.Info
	Root            string `json:"root"`
	TemplatesRoot   string `json:"templatesRoot"`
	DockerVersion   string `json:"dockerVersion,omitempty"`
	DockerError     string `json:"dockerError,omitempty"`
	MemoryReserve   int64  `json:"memoryReserve"`
	MemoryBudget    int64  `json:"memoryBudget"`
	MemoryCommitted int64  `json:"memoryCommitted"`
	MemoryPlanned   int64  `json:"memoryPlanned"`
	InstanceCount   int    `json:"instanceCount"`
}

func isUp(s instance.State) bool {
	switch s {
	case instance.StateRunning, instance.StateStarting, instance.StateProvisioning, instance.StateUpdating:
		return true
	}
	return false
}

const (
	OpProvision = "provision"
	OpUpdate    = "update"
	OpStart     = "start"
	OpStop      = "stop"
)

func (m *Manager) operation(name string) *instance.Operation {
	m.mu.Lock()
	defer m.mu.Unlock()
	op, ok := m.ops[name]
	if !ok {
		return nil
	}
	cp := *op
	return &cp
}

func (m *Manager) beginOp(name, kind, code string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if cur, ok := m.ops[name]; ok && cur.Error == "" {
		return fmt.Errorf("%q is already in %s", name, cur.Kind)
	}
	m.ops[name] = &instance.Operation{Kind: kind, Code: code, StartedAt: m.now()}
	return nil
}

func (m *Manager) progress(name, code, msg string, pct *int) {
	m.mu.Lock()
	if op, ok := m.ops[name]; ok {
		op.Code = code
		op.Message = msg
		op.Percent = pct
	}
	m.mu.Unlock()
	m.hub.Publish(Event{Type: "instance.progress", Instance: name, Message: msg})
}

func (m *Manager) endOp(name string, err error) {
	m.mu.Lock()
	if err != nil {
		if op, ok := m.ops[name]; ok {
			op.Error = err.Error()
		}
	} else {
		delete(m.ops, name)
	}
	m.mu.Unlock()

	ev := Event{Type: "instance.changed", Instance: name}
	if err != nil {
		ev.Type = "instance.failed"
		ev.Message = err.Error()
	}
	m.hub.Publish(ev)
}

func (m *Manager) ClearError(name string) {
	m.mu.Lock()
	if op, ok := m.ops[name]; ok && op.Error != "" {
		delete(m.ops, name)
	}
	m.mu.Unlock()
	m.hub.Publish(Event{Type: "instance.changed", Instance: name})
}

type ErrBudget struct {
	Requested, Committed, Budget int64
	Instance                     string
}

func (e *ErrBudget) Error() string {
	return fmt.Sprintf(
		"%s asks for %s, but only %s is free in the %s budget (running instances already use %s)",
		e.Instance,
		instance.FormatMemory(e.Requested),
		instance.FormatMemory(max64(0, e.Budget-e.Committed)),
		instance.FormatMemory(e.Budget),
		instance.FormatMemory(e.Committed),
	)
}

func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

type ErrPortTaken struct {
	Port  int
	Proto string
	Owner string
}

func (e *ErrPortTaken) Error() string {
	return fmt.Sprintf("port %d/%s is already used by %s", e.Port, e.Proto, e.Owner)
}

// one folder per volume next to the compose, and a repeated last name carries the whole path
func mountsFor(volumes []template.Volume) []instance.Mount {
	mounts := make([]instance.Mount, 0, len(volumes))
	taken := make(map[string]bool, len(volumes))
	for _, v := range volumes {
		host := hostDirFor(v.Container)
		if taken[host] {
			host = "./" + strings.ReplaceAll(strings.Trim(v.Container, "/"), "/", "-")
		}
		taken[host] = true
		mounts = append(mounts, instance.Mount{Host: host, Container: v.Container})
	}
	return mounts
}

// the folder of a mount is named after the last piece of the path, next to the compose file
func hostDirFor(dir string) string {
	name := path.Base(strings.TrimSuffix(dir, "/"))
	if name == "" || name == "." || name == "/" {
		return "./data"
	}
	return "./" + name
}

func (m *Manager) checkBudget(ctx context.Context, spec instance.Spec) error {
	want, err := instance.ParseMemory(spec.MemoryLimit)
	if err != nil {
		return err
	}
	if want == 0 {
		return nil
	}
	info, err := m.sys.Read(m.diskPath())
	if err != nil {
		return err
	}
	budget := info.MemoryTotal - m.reserve

	var committed int64
	list, err := m.List(ctx)
	if err != nil {
		return err
	}
	for _, i := range list {
		if i.Name == spec.Name || !isUp(i.State) {
			continue
		}
		n, _ := instance.ParseMemory(i.MemoryLimit)
		committed += n
	}
	if committed+want > budget {
		return &ErrBudget{Requested: want, Committed: committed, Budget: budget, Instance: spec.Name}
	}
	return nil
}

func (m *Manager) checkPorts(ctx context.Context, spec instance.Spec) error {
	specs, err := m.store.List()
	if err != nil {
		return err
	}
	taken := map[string]string{}
	for _, other := range specs {
		if other.Name == spec.Name {
			continue
		}
		for _, p := range other.Ports {
			taken[fmt.Sprintf("%d/%s", p.Host, p.Protocol)] = other.Name
		}
	}
	for port, name := range m.portsHeldOutside(ctx, spec.Name) {
		if _, ok := taken[port]; !ok {
			taken[port] = name
		}
	}
	seen := map[string]bool{}
	for _, p := range spec.Ports {
		key := fmt.Sprintf("%d/%s", p.Host, p.Protocol)
		if owner, ok := taken[key]; ok {
			return &ErrPortTaken{Port: p.Host, Proto: p.Protocol, Owner: owner}
		}
		if seen[key] {
			return &ErrPortTaken{Port: p.Host, Proto: p.Protocol, Owner: spec.Name}
		}
		seen[key] = true
	}
	return nil
}

// a container the panel did not create holds the port just as hard, and docker ps is the only place it shows
func (m *Manager) portsHeldOutside(ctx context.Context, skip string) map[string]string {
	containers, err := m.docker.PSAll(ctx)
	if err != nil {
		slog.Debug("could not list the host containers to check the ports", "err", err)
		return nil
	}
	out := map[string]string{}
	for _, c := range containers {
		if c.Name == skip || (c.State != "running" && c.State != "restarting") {
			continue
		}
		for _, p := range c.Ports {
			out[fmt.Sprintf("%d/%s", p.Host, p.Protocol)] = c.Name
		}
	}
	return out
}

func (m *Manager) SuggestPort(base int, proto string) int {
	specs, err := m.store.List()
	if err != nil {
		return base
	}
	taken := map[int]bool{}
	for _, s := range specs {
		for _, p := range s.Ports {
			if p.Protocol == proto {
				taken[p.Host] = true
			}
		}
	}
	for port := base; port < base+200 && port < 65536; port++ {
		if !taken[port] {
			return port
		}
	}
	return base
}

func (m *Manager) BuildSpec(req SpecRequest) (instance.Spec, error) {
	if err := instance.ValidateName(req.Name); err != nil {
		return instance.Spec{}, err
	}
	tmpl, ok := m.templates.Get(req.TemplateID)
	if !ok {
		return instance.Spec{}, fmt.Errorf("unknown template: %q", req.TemplateID)
	}

	image := strings.TrimSpace(req.Image)
	if image == "" {
		image = tmpl.Image
	}
	if image == "" {
		return instance.Spec{}, errors.New("the custom template needs an image name")
	}
	validated, err := tmpl.Validate(req.Values)
	if err != nil {
		return instance.Spec{}, err
	}
	env, args := tmpl.SplitValues(validated)

	secretKeys := req.SecretKeys
	if tmpl.ID != template.CustomID {
		secretKeys = nil
		for _, f := range tmpl.Fields {
			if f.Secret {
				secretKeys = append(secretKeys, f.Key)
			}
		}
	}
	sort.Strings(secretKeys)

	ports := req.Ports
	if len(ports) == 0 {
		for _, p := range tmpl.Ports {
			if p.Optional {
				continue
			}
			ports = append(ports, instance.PortBinding{
				Host:      m.SuggestPort(p.Container, p.Protocol),
				Container: p.Container,
				Protocol:  p.Protocol,
				Label:     p.Label,
			})
		}
	}
	for i, p := range ports {
		if p.Host < 1 || p.Host > 65535 || p.Container < 1 || p.Container > 65535 {
			return instance.Spec{}, fmt.Errorf("invalid port: %s", p)
		}
		if p.Protocol != "tcp" && p.Protocol != "udp" {
			ports[i].Protocol = "tcp"
		}
	}

	mounts := req.Mounts
	if len(mounts) == 0 {
		mounts = mountsFor(tmpl.Volumes)
	}

	memLimit := strings.TrimSpace(req.MemoryLimit)
	if memLimit == "" {
		memLimit = tmpl.DefaultMemory
	}
	want, err := instance.ParseMemory(memLimit)
	if err != nil {
		return instance.Spec{}, err
	}
	if min, err := instance.ParseMemory(tmpl.MinMemory); err == nil && min > 0 && want < min {
		return instance.Spec{}, fmt.Errorf(
			"%s asks for at least %s of RAM, %s does not start (the image dies with exit 137)",
			tmpl.Name, tmpl.MinMemory, memLimit)
	}

	cpus := req.CPUs
	if cpus <= 0 {
		cpus = tmpl.DefaultCPUs
	}
	restart := req.Restart
	if restart == "" {
		restart = "unless-stopped"
	}

	return instance.Spec{
		Name:             req.Name,
		TemplateID:       tmpl.ID,
		Category:         string(tmpl.Category),
		Image:            image,
		Env:              env,
		Command:          args,
		SecretKeys:       secretKeys,
		Ports:            ports,
		Mounts:           mounts,
		MemoryLimit:      instance.FormatMemory(want),
		CPUs:             cpus,
		Restart:          restart,
		StopGraceSeconds: tmpl.StopGraceSeconds,
		UpdatedAt:        m.now(),
	}, nil
}

type SpecRequest struct {
	Name        string                 `json:"name"`
	TemplateID  string                 `json:"templateId"`
	Image       string                 `json:"image,omitempty"`
	Values      map[string]string      `json:"values"`
	Ports       []instance.PortBinding `json:"ports,omitempty"`
	Mounts      []instance.Mount       `json:"mounts,omitempty"`
	MemoryLimit string                 `json:"memoryLimit,omitempty"`
	CPUs        float64                `json:"cpus,omitempty"`
	Restart     string                 `json:"restart,omitempty"`
	SecretKeys  []string               `json:"secretKeys,omitempty"`
	Start       bool                   `json:"start,omitempty"`
}

func (m *Manager) PreviewCompose(req SpecRequest) ([]byte, error) {
	spec, err := m.BuildSpec(req)
	if err != nil {
		return nil, err
	}
	return compose.Render(spec)
}

func (m *Manager) Create(ctx context.Context, req SpecRequest) (instance.Spec, error) {
	spec, err := m.BuildSpec(req)
	if err != nil {
		return instance.Spec{}, err
	}
	if m.store.Exists(spec.Name) {
		return instance.Spec{}, fmt.Errorf("%q: %w", spec.Name, store.ErrExists)
	}
	if err := m.checkPorts(ctx, spec); err != nil {
		return instance.Spec{}, err
	}
	if req.Start {
		if err := m.checkBudget(ctx, spec); err != nil {
			return instance.Spec{}, err
		}
	}
	if err := m.store.Create(spec); err != nil {
		return instance.Spec{}, err
	}
	m.hub.Publish(Event{Type: "instance.created", Instance: spec.Name})

	if req.Start {
		if err := m.beginOp(spec.Name, OpProvision, "preparing"); err != nil {
			return spec, err
		}
		go m.provision(spec.Name)
	}
	return spec, nil
}

func (m *Manager) provision(name string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	dir := m.store.Dir(name)
	err := m.docker.Pull(ctx, dir, func(line string) {
		m.progress(name, "", line, nil)
	})
	if err == nil {
		m.progress(name, "creating", "", nil)
		err = m.docker.Up(ctx, dir)
	}
	m.endOp(name, err)
}

type ExternalError struct{ Name string }

func (e *ExternalError) Error() string {
	return fmt.Sprintf("%q is an external container, the panel only starts, stops and shows the console", e.Name)
}

var ErrExternal = errors.New("external container")

func (e *ExternalError) Is(target error) bool { return target == ErrExternal }

func (m *Manager) notManaged(ctx context.Context, name string, err error) error {
	if _, ok := m.external(ctx, name); ok {
		return &ExternalError{Name: name}
	}
	return err
}

func (m *Manager) Update(ctx context.Context, name string, req SpecRequest) (instance.Spec, error) {
	old, err := m.store.Get(name)
	if err != nil {
		if inst, ok := m.external(ctx, name); ok {
			return m.updateExternal(ctx, inst, req)
		}
		return instance.Spec{}, err
	}
	req.Name = name
	if req.TemplateID == "" {
		req.TemplateID = old.TemplateID
	}
	// what the request leaves out keeps the value saved, the template would drop a volume or a secret
	if req.Mounts == nil {
		req.Mounts = old.Mounts
	}
	if req.SecretKeys == nil {
		req.SecretKeys = old.SecretKeys
	}
	spec, err := m.BuildSpec(req)
	if err != nil {
		return instance.Spec{}, err
	}
	spec.Archived = old.Archived
	if err := m.checkPorts(ctx, spec); err != nil {
		return instance.Spec{}, err
	}

	inst, err := m.Get(ctx, name)
	if err != nil {
		return instance.Spec{}, err
	}
	wasUp := isUp(inst.State)
	if wasUp {
		if err := m.checkBudget(ctx, spec); err != nil {
			return instance.Spec{}, err
		}
	}
	if err := m.store.Update(spec); err != nil {
		return instance.Spec{}, err
	}

	if wasUp && NeedsRecreate(old, spec) {
		if err := m.beginOp(name, OpUpdate, "recreating"); err != nil {
			return spec, err
		}
		go m.recreate(name)
	} else {
		m.hub.Publish(Event{Type: "instance.changed", Instance: name})
	}
	return spec, nil
}

func (m *Manager) recreate(name string) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()

	dir := m.store.Dir(name)
	m.progress(name, "stopping", "", nil)
	err := m.docker.Down(ctx, dir)
	if err == nil {
		m.progress(name, "starting_new_config", "", nil)
		err = m.docker.Up(ctx, dir)
	}
	m.endOp(name, err)
}

func NeedsRecreate(old, new instance.Spec) bool {
	if old.Image != new.Image ||
		old.MemoryLimit != new.MemoryLimit ||
		old.CPUs != new.CPUs ||
		old.Restart != new.Restart ||
		old.StopGraceSeconds != new.StopGraceSeconds {
		return true
	}
	if len(old.Command) != len(new.Command) {
		return true
	}
	for i := range new.Command {
		if old.Command[i] != new.Command[i] {
			return true
		}
	}
	if len(old.Env) != len(new.Env) {
		return true
	}
	for k, v := range new.Env {
		if old.Env[k] != v {
			return true
		}
	}
	if len(old.Ports) != len(new.Ports) {
		return true
	}
	for i := range new.Ports {
		if old.Ports[i] != new.Ports[i] {
			return true
		}
	}
	if len(old.Mounts) != len(new.Mounts) {
		return true
	}
	for i := range new.Mounts {
		if old.Mounts[i] != new.Mounts[i] {
			return true
		}
	}
	return false
}

func RecreateFields(old, new instance.Spec) []string {
	var out []string
	if old.Image != new.Image {
		out = append(out, "image")
	}
	if old.MemoryLimit != new.MemoryLimit {
		out = append(out, "RAM limit")
	}
	if old.CPUs != new.CPUs {
		out = append(out, "CPUs")
	}
	if old.Restart != new.Restart {
		out = append(out, "restart policy")
	}
	for k, v := range new.Env {
		if old.Env[k] != v {
			out = append(out, k)
		}
	}
	for k := range old.Env {
		if _, ok := new.Env[k]; !ok {
			out = append(out, k+" (removed)")
		}
	}
	if len(old.Ports) != len(new.Ports) {
		out = append(out, "ports")
	} else {
		for i := range new.Ports {
			if old.Ports[i] != new.Ports[i] {
				out = append(out, "ports")
				break
			}
		}
	}
	sort.Strings(out)
	return out
}

func (m *Manager) Start(ctx context.Context, name string) error {
	spec, err := m.store.Get(name)
	if err != nil {
		if _, ok := m.external(ctx, name); ok {
			return m.externalAction(name, "start", OpStart, "starting")
		}
		return err
	}
	if spec.Archived {
		return fmt.Errorf("%q is archived, restore it before starting", name)
	}
	if err := m.checkBudget(ctx, spec); err != nil {
		return err
	}
	if err := m.beginOp(name, OpStart, "starting"); err != nil {
		return err
	}
	go func() {
		c, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		defer cancel()
		m.endOp(name, m.docker.Up(c, m.store.Dir(name)))
	}()
	return nil
}

// runs in a goroutine because docker stop waits for the container stop_grace_period
func (m *Manager) externalAction(name, verb, kind, code string) error {
	if err := m.beginOp(name, kind, code); err != nil {
		return err
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		m.endOp(name, m.docker.ContainerAction(ctx, name, verb))
	}()
	return nil
}

func (m *Manager) UpdateImage(ctx context.Context, name string) error {
	spec, err := m.store.Get(name)
	if err != nil {
		inst, ok := m.external(ctx, name)
		if !ok {
			return err
		}
		// the compose CLI runs inside the panel, so it needs the file, not only the reported path
		if !inst.Editable {
			return &ExternalError{Name: name}
		}
		if err := m.beginOp(name, OpUpdate, "checking_update"); err != nil {
			return err
		}
		go m.pullExternal(inst)
		return nil
	}
	if spec.Archived {
		return fmt.Errorf("%q is archived, restore it before updating", name)
	}
	if err := m.beginOp(name, OpUpdate, "checking_update"); err != nil {
		return err
	}
	go m.pullAndRecreate(name, spec)
	return nil
}

func (m *Manager) pullAndRecreate(name string, spec instance.Spec) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	dir := m.store.Dir(name)
	before, _ := m.docker.ImageID(ctx, spec.Image)

	err := m.docker.Pull(ctx, dir, func(line string) {
		m.progress(name, "", line, nil)
	})
	if err != nil {
		m.endOp(name, err)
		return
	}

	after, _ := m.docker.ImageID(ctx, spec.Image)
	if before != "" && before == after {
		m.endOp(name, nil)
		m.hub.Publish(Event{
			Type:     "instance.uptodate",
			Instance: name,
			Message:  fmt.Sprintf("%s is already on the newest image", name),
		})
		return
	}

	m.progress(name, "recreating_new_image", "", nil)
	err = m.docker.Up(ctx, dir)
	m.endOp(name, err)
	if err == nil {
		m.hub.Publish(Event{
			Type:     "instance.updated",
			Instance: name,
			Message:  fmt.Sprintf("%s was updated, the world in the volumes was kept", name),
		})
	}
}

func (m *Manager) Stop(ctx context.Context, name string) error {
	if _, err := m.store.Get(name); err != nil {
		if _, ok := m.external(ctx, name); ok {
			return m.externalAction(name, "stop", OpStop, "stopping")
		}
		return err
	}
	if err := m.beginOp(name, OpStop, "stopping"); err != nil {
		return err
	}
	go func() {
		c, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		m.endOp(name, m.docker.Down(c, m.store.Dir(name)))
	}()
	return nil
}

func (m *Manager) Restart(ctx context.Context, name string) error {
	if _, err := m.store.Get(name); err != nil {
		if _, ok := m.external(ctx, name); ok {
			return m.externalAction(name, "restart", OpStart, "restarting")
		}
		return err
	}
	if err := m.beginOp(name, OpStart, "restarting"); err != nil {
		return err
	}
	go func() {
		c, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		m.endOp(name, m.docker.Restart(c, m.store.Dir(name)))
	}()
	return nil
}

func (m *Manager) SetArchived(ctx context.Context, name string, archived bool) error {
	spec, err := m.store.Get(name)
	if err != nil {
		return m.notManaged(ctx, name, err)
	}
	if archived {
		if err := m.docker.Down(ctx, m.store.Dir(name)); err != nil {
			return err
		}
	}
	spec.Archived = archived
	if err := m.store.Update(spec); err != nil {
		return err
	}
	m.hub.Publish(Event{Type: "instance.changed", Instance: name})
	return nil
}

func (m *Manager) Delete(ctx context.Context, name string, keepData bool) error {
	if _, err := m.store.Get(name); err != nil {
		return m.notManaged(ctx, name, err)
	}
	if err := m.docker.Down(ctx, m.store.Dir(name)); err != nil {
		var de *dockerx.Error
		if !errors.As(err, &de) {
			return err
		}
	}
	if err := m.store.Delete(name, keepData); err != nil {
		return err
	}
	m.mu.Lock()
	delete(m.ops, name)
	m.mu.Unlock()
	m.forgetDNS(name)
	m.hub.Publish(Event{Type: "instance.deleted", Instance: name})
	return nil
}

func (m *Manager) Logs(ctx context.Context, name string, tail int, follow bool) (readCloser, error) {
	if _, err := m.store.Get(name); err != nil {
		if _, ok := m.external(ctx, name); ok {
			return m.docker.ContainerLogs(ctx, name, tail, follow)
		}
		return nil, err
	}
	return m.docker.Logs(ctx, m.store.Dir(name), tail, follow)
}

type readCloser = interface {
	Read([]byte) (int, error)
	Close() error
}

func (m *Manager) Compose(ctx context.Context, name string) ([]byte, error) {
	raw, err := m.store.ReadCompose(name)
	if err == nil {
		return raw, nil
	}
	if inst, ok := m.external(ctx, name); ok && inst.ComposeFile != "" {
		return os.ReadFile(inst.ComposeFile)
	}
	return nil, err
}

// SearchImages goes through the daemon, so a failure there is a docker failure like any other
func (m *Manager) SearchImages(ctx context.Context, term string, limit int) ([]dockerx.ImageHit, error) {
	term = strings.TrimSpace(term)
	if term == "" {
		return []dockerx.ImageHit{}, nil
	}
	hits, err := m.docker.SearchImages(ctx, term, limit)
	if err != nil {
		return nil, err
	}
	if hits == nil {
		hits = []dockerx.ImageHit{}
	}
	return hits, nil
}

// ImageTags asks the Hub, the daemon search reports no tags, so the panel reaches the network
func (m *Manager) ImageTags(ctx context.Context, repo string) ([]string, error) {
	if m.registry == nil {
		return nil, registry.ErrNotHub
	}
	repo = strings.TrimSpace(repo)
	if repo == "" {
		return []string{}, nil
	}
	return m.registry.Tags(ctx, repo)
}

func (m *Manager) Store() *store.Store { return m.store }
