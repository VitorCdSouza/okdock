package compose

import (
	"bytes"
	"fmt"
	"sort"
	"strconv"

	"gopkg.in/yaml.v3"

	"github.com/VitorCdSouza/okdock/api/internal/instance"
)

// Apply writes one service back and leaves the rest of the file as the author left it
func (p *Project) Apply(service string, spec instance.Spec) error {
	body := p.serviceNode(service)
	if body == nil {
		return fmt.Errorf("service %q is not in this compose file", service)
	}

	setScalar(body, "image", spec.Image)
	setScalar(body, "restart", spec.Restart)
	if spec.StopGraceSeconds > 0 {
		setScalar(body, "stop_grace_period", strconv.Itoa(spec.StopGraceSeconds)+"s")
	} else {
		removeKey(body, "stop_grace_period")
	}

	setSequence(body, "command", spec.Command)

	ports := make([]string, 0, len(spec.Ports))
	for _, port := range spec.Ports {
		ports = append(ports, port.String())
	}
	setSequence(body, "ports", ports)

	volumes := make([]string, 0, len(spec.Mounts))
	for _, m := range spec.Mounts {
		volumes = append(volumes, m.Host+":"+m.Container)
	}
	setSequence(body, "volumes", volumes)

	setPairs(body, "environment", spec.Env, spec.SecretKeys)
	setLimits(body, spec)
	return nil
}

// the file dialect wins: a service already using mem_limit keeps mem_limit
func setLimits(body *yaml.Node, spec instance.Spec) {
	limits := path(body, "deploy", "resources", "limits")
	if limits == nil && (mapValue(body, "mem_limit") != nil || mapValue(body, "cpus") != nil) {
		setScalar(body, "mem_limit", spec.MemoryLimit)
		setScalar(body, "cpus", formatCPUs(spec.CPUs))
		return
	}
	if limits == nil {
		if spec.MemoryLimit == "" && spec.CPUs <= 0 {
			return
		}
		limits = ensurePath(body, "deploy", "resources", "limits")
	}
	setScalar(limits, "memory", spec.MemoryLimit)
	setScalar(limits, "cpus", formatCPUs(spec.CPUs))
}

func formatCPUs(v float64) string {
	if v <= 0 {
		return ""
	}
	return strconv.FormatFloat(v, 'f', -1, 64)
}

func (p *Project) Bytes() ([]byte, error) {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(p.node); err != nil {
		return nil, fmt.Errorf("writing the compose file: %w", err)
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (p *Project) serviceNode(name string) *yaml.Node {
	services := path(docRoot(p.node), "services")
	if services == nil || services.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(services.Content); i += 2 {
		body := services.Content[i+1]
		if body.Kind != yaml.MappingNode {
			continue
		}
		if services.Content[i].Value == name || scalar(body, "container_name") == name {
			return body
		}
	}
	return nil
}

func setScalar(n *yaml.Node, key, value string) {
	if value == "" {
		removeKey(n, key)
		return
	}
	if cur := mapValue(n, key); cur != nil && cur.Kind == yaml.ScalarNode {
		cur.Tag, cur.Value = "!!str", value
		return
	}
	replaceOrAppend(n, key, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value})
}

func setSequence(n *yaml.Node, key string, values []string) {
	if len(values) == 0 {
		removeKey(n, key)
		return
	}
	seq := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
	for _, v := range values {
		item := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: v}
		if key == "ports" {
			item.Style = yaml.DoubleQuotedStyle
		}
		seq.Content = append(seq.Content, item)
	}
	replaceOrAppend(n, key, seq)
}

// keeps KEY=value when the file already writes it that way, and leaves a secret to the .env
func setPairs(n *yaml.Node, key string, values map[string]string, secretKeys []string) {
	secret := make(map[string]bool, len(secretKeys))
	for _, k := range secretKeys {
		secret[k] = true
	}
	keys := make([]string, 0, len(values))
	for k := range values {
		if !secret[k] {
			keys = append(keys, k)
		}
	}
	if len(keys) == 0 {
		removeKey(n, key)
		return
	}
	sort.Strings(keys)

	cur := mapValue(n, key)
	if cur != nil && cur.Kind == yaml.SequenceNode {
		seq := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
		for _, k := range keys {
			seq.Content = append(seq.Content, &yaml.Node{
				Kind: yaml.ScalarNode, Tag: "!!str", Value: k + "=" + values[k],
			})
		}
		replaceOrAppend(n, key, seq)
		return
	}
	m := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	for _, k := range keys {
		m.Content = append(m.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: k},
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: values[k]})
	}
	replaceOrAppend(n, key, m)
}

func ensurePath(n *yaml.Node, keys ...string) *yaml.Node {
	for _, k := range keys {
		next := mapValue(n, k)
		if next == nil || next.Kind != yaml.MappingNode {
			next = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
			replaceOrAppend(n, k, next)
		}
		n = next
	}
	return n
}

func replaceOrAppend(n *yaml.Node, key string, value *yaml.Node) {
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == key {
			old := n.Content[i+1]
			value.HeadComment, value.LineComment, value.FootComment =
				old.HeadComment, old.LineComment, old.FootComment
			n.Content[i+1] = value
			return
		}
	}
	n.Content = append(n.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}, value)
}

func removeKey(n *yaml.Node, key string) {
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == key {
			n.Content = append(n.Content[:i], n.Content[i+2:]...)
			return
		}
	}
}
