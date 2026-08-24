package compose

import (
	"fmt"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/VitorCdSouza/okdock/api/internal/instance"
)

// Project is one compose file read back, node kept so Apply rewrites only the panel keys
type Project struct {
	Name     string
	Services []Service
	// what the panel cannot express, so the caller can refuse to write back
	Unsupported []string

	node *yaml.Node
}

type Service struct {
	Name            string
	ContainerName   string
	Image           string
	Command         []string
	Restart         string
	StopGraceSecond int
	Ports           []instance.PortBinding
	Environment     map[string]string
	EnvFile         []string
	Volumes         []instance.Mount
	MemoryLimit     string
	CPUs            float64
	Labels          map[string]string
}

func Parse(data []byte) (*Project, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("reading the compose file: %w", err)
	}
	root := docRoot(&doc)
	if root == nil || root.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("reading the compose file: no top level mapping")
	}

	p := &Project{node: &doc}
	if n := mapValue(root, "name"); n != nil {
		p.Name = n.Value
	}
	for _, key := range []string{"include", "extends"} {
		if mapValue(root, key) != nil {
			p.Unsupported = append(p.Unsupported, key)
		}
	}

	services := mapValue(root, "services")
	if services == nil || services.Kind != yaml.MappingNode {
		return p, nil
	}
	for i := 0; i+1 < len(services.Content); i += 2 {
		name, body := services.Content[i].Value, services.Content[i+1]
		if body.Kind != yaml.MappingNode {
			continue
		}
		svc, unsupported := parseService(name, body)
		p.Services = append(p.Services, svc)
		p.Unsupported = append(p.Unsupported, unsupported...)
	}
	return p, nil
}

func parseService(name string, body *yaml.Node) (Service, []string) {
	svc := Service{Name: name, Environment: map[string]string{}, Labels: map[string]string{}}
	var unsupported []string

	svc.ContainerName = scalar(body, "container_name")
	svc.Image = scalar(body, "image")
	svc.Restart = scalar(body, "restart")
	svc.Command = stringList(mapValue(body, "command"))
	svc.EnvFile = stringList(mapValue(body, "env_file"))
	svc.StopGraceSecond = parseSeconds(scalar(body, "stop_grace_period"))

	if n := mapValue(body, "extends"); n != nil {
		unsupported = append(unsupported, name+".extends")
	}

	for k, v := range pairs(mapValue(body, "environment")) {
		svc.Environment[k] = v
	}
	for k, v := range pairs(mapValue(body, "labels")) {
		svc.Labels[k] = v
	}

	if n := mapValue(body, "ports"); n != nil {
		for _, item := range n.Content {
			p, ok := parsePort(item)
			if !ok {
				unsupported = append(unsupported, name+".ports")
				continue
			}
			svc.Ports = append(svc.Ports, p)
		}
	}
	if n := mapValue(body, "volumes"); n != nil {
		for _, item := range n.Content {
			m, ok := parseVolume(item)
			if !ok {
				unsupported = append(unsupported, name+".volumes")
				continue
			}
			svc.Volumes = append(svc.Volumes, m)
		}
	}

	svc.MemoryLimit = scalar(body, "mem_limit")
	if raw := scalar(body, "cpus"); raw != "" {
		svc.CPUs, _ = strconv.ParseFloat(raw, 64)
	}
	if limits := path(body, "deploy", "resources", "limits"); limits != nil {
		if v := scalar(limits, "memory"); v != "" {
			svc.MemoryLimit = v
		}
		if v := scalar(limits, "cpus"); v != "" {
			svc.CPUs, _ = strconv.ParseFloat(v, 64)
		}
	}
	return svc, unsupported
}

// Spec turns one service into what the panel edits, the caller fills what the file cannot say
func (s Service) Spec() instance.Spec {
	name := s.ContainerName
	if name == "" {
		name = s.Name
	}
	spec := instance.Spec{
		Name:             name,
		TemplateID:       s.Labels[Label+".template"],
		Category:         s.Labels[Label+".category"],
		Image:            s.Image,
		Env:              map[string]string{},
		Command:          s.Command,
		Ports:            []instance.PortBinding{},
		Mounts:           []instance.Mount{},
		MemoryLimit:      s.MemoryLimit,
		CPUs:             s.CPUs,
		Restart:          s.Restart,
		StopGraceSeconds: s.StopGraceSecond,
	}
	for k, v := range s.Environment {
		spec.Env[k] = v
	}
	spec.Ports = append(spec.Ports, s.Ports...)
	spec.Mounts = append(spec.Mounts, s.Volumes...)
	return spec
}

func (p *Project) Service(name string) (Service, bool) {
	for _, s := range p.Services {
		if s.Name == name || (s.ContainerName != "" && s.ContainerName == name) {
			return s, true
		}
	}
	return Service{}, false
}

// docRoot unwraps the document node yaml.Unmarshal wraps everything in
func docRoot(n *yaml.Node) *yaml.Node {
	if n.Kind == yaml.DocumentNode {
		if len(n.Content) == 0 {
			return nil
		}
		return n.Content[0]
	}
	return n
}

func mapValue(n *yaml.Node, key string) *yaml.Node {
	if n == nil || n.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == key {
			return n.Content[i+1]
		}
	}
	return nil
}

func path(n *yaml.Node, keys ...string) *yaml.Node {
	for _, k := range keys {
		n = mapValue(n, k)
		if n == nil {
			return nil
		}
	}
	return n
}

func scalar(n *yaml.Node, key string) string {
	v := mapValue(n, key)
	if v == nil || v.Kind != yaml.ScalarNode {
		return ""
	}
	return v.Value
}

// a compose list is either a real list or, for command, a single string
func stringList(n *yaml.Node) []string {
	if n == nil {
		return nil
	}
	if n.Kind == yaml.ScalarNode {
		if n.Value == "" {
			return nil
		}
		return strings.Fields(n.Value)
	}
	var out []string
	for _, item := range n.Content {
		if item.Kind == yaml.ScalarNode {
			out = append(out, item.Value)
		}
	}
	return out
}

// environment and labels accept both a mapping and a KEY=value list
func pairs(n *yaml.Node) map[string]string {
	out := map[string]string{}
	if n == nil {
		return out
	}
	switch n.Kind {
	case yaml.MappingNode:
		for i := 0; i+1 < len(n.Content); i += 2 {
			out[n.Content[i].Value] = n.Content[i+1].Value
		}
	case yaml.SequenceNode:
		for _, item := range n.Content {
			if item.Kind != yaml.ScalarNode {
				continue
			}
			k, v, ok := strings.Cut(item.Value, "=")
			if !ok {
				out[k] = ""
				continue
			}
			out[k] = v
		}
	}
	return out
}

func parsePort(n *yaml.Node) (instance.PortBinding, bool) {
	switch n.Kind {
	case yaml.ScalarNode:
		return parsePortString(n.Value)
	case yaml.MappingNode:
		target, err := strconv.Atoi(scalar(n, "target"))
		if err != nil {
			return instance.PortBinding{}, false
		}
		published := scalar(n, "published")
		host, err := strconv.Atoi(published)
		if err != nil {
			return instance.PortBinding{}, false
		}
		proto := scalar(n, "protocol")
		if proto == "" {
			proto = "tcp"
		}
		return instance.PortBinding{Host: host, Container: target, Protocol: proto}, true
	}
	return instance.PortBinding{}, false
}

// [ip:][host:]container[/proto], and a range is not a binding the panel edits
func parsePortString(raw string) (instance.PortBinding, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.Contains(raw, "-") {
		return instance.PortBinding{}, false
	}
	proto := "tcp"
	if body, p, ok := strings.Cut(raw, "/"); ok {
		raw, proto = body, p
	}
	parts := strings.Split(raw, ":")
	if len(parts) > 3 {
		return instance.PortBinding{}, false
	}
	container, err := strconv.Atoi(parts[len(parts)-1])
	if err != nil {
		return instance.PortBinding{}, false
	}
	if len(parts) == 1 {
		// no published port, docker picks one, so there is nothing to edit
		return instance.PortBinding{}, false
	}
	host, err := strconv.Atoi(parts[len(parts)-2])
	if err != nil {
		return instance.PortBinding{}, false
	}
	return instance.PortBinding{Host: host, Container: container, Protocol: proto}, true
}

func parseVolume(n *yaml.Node) (instance.Mount, bool) {
	switch n.Kind {
	case yaml.ScalarNode:
		parts := strings.Split(n.Value, ":")
		if len(parts) < 2 {
			return instance.Mount{}, false
		}
		return instance.Mount{Host: parts[0], Container: parts[1]}, true
	case yaml.MappingNode:
		source, target := scalar(n, "source"), scalar(n, "target")
		if source == "" || target == "" {
			return instance.Mount{}, false
		}
		return instance.Mount{Host: source, Container: target}, true
	}
	return instance.Mount{}, false
}

func parseSeconds(raw string) int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}
	if n, err := strconv.Atoi(raw); err == nil {
		return n
	}
	if n, err := strconv.Atoi(strings.TrimSuffix(raw, "s")); err == nil {
		return n
	}
	if n, err := strconv.Atoi(strings.TrimSuffix(raw, "m")); err == nil {
		return n * 60
	}
	return 0
}
