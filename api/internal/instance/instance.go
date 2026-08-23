package instance

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type State string

const (
	StateStopped      State = "stopped"
	StateProvisioning State = "provisioning"
	StateStarting     State = "starting"
	StateRunning      State = "running"
	StateUpdating     State = "updating"
	StateError        State = "error"
	StateArchived     State = "archived"
)

var AllStates = []State{
	StateStopped, StateProvisioning, StateStarting,
	StateRunning, StateUpdating, StateArchived, StateError,
}

type PortBinding struct {
	Host      int    `json:"host"`
	Container int    `json:"container"`
	Protocol  string `json:"protocol"`
	Label     string `json:"label,omitempty"`
}

func (p PortBinding) String() string {
	return fmt.Sprintf("%d:%d/%s", p.Host, p.Container, p.Protocol)
}

type Mount struct {
	Host      string `json:"host"`
	Container string `json:"container"`
	Data      bool   `json:"data,omitempty"`
}

type Spec struct {
	Name             string            `json:"name"`
	TemplateID       string            `json:"templateId"`
	Category         string            `json:"category"`
	Image            string            `json:"image"`
	Env              map[string]string `json:"env"`
	SecretKeys       []string          `json:"secretKeys,omitempty"`
	Command          []string          `json:"command,omitempty"`
	Ports            []PortBinding     `json:"ports"`
	Mounts           []Mount           `json:"mounts"`
	MemoryLimit      string            `json:"memoryLimit"`
	CPUs             float64           `json:"cpus"`
	Restart          string            `json:"restart"`
	StopGraceSeconds int               `json:"stopGraceSeconds"`
	Archived         bool              `json:"archived,omitempty"`
	CreatedAt        time.Time         `json:"createdAt"`
	UpdatedAt        time.Time         `json:"updatedAt"`
}

func (s *Spec) UnmarshalJSON(raw []byte) error {
	type alias Spec
	var v struct {
		alias
		LegacyProviderID string `json:"providerId"`
		LegacyGame       string `json:"game"`
	}
	if err := json.Unmarshal(raw, &v); err != nil {
		return err
	}
	*s = Spec(v.alias)
	if s.TemplateID == "" {
		s.TemplateID = v.LegacyProviderID
	}
	if s.Category == "" && v.LegacyGame != "" {
		s.Category = "games"
	}
	return nil
}

type DNS struct {
	Domain    string    `json:"domain"`
	Hostname  string    `json:"hostname"`
	LastIP    string    `json:"lastIp,omitempty"`
	LastSync  time.Time `json:"lastSync,omitempty"`
	LastError string    `json:"lastError,omitempty"`
}

type Stats struct {
	CPUPercent  float64 `json:"cpuPercent"`
	MemoryBytes int64   `json:"memoryBytes"`
	MemoryLimit int64   `json:"memoryLimit"`
}

type Instance struct {
	Spec
	Dir       string     `json:"dir"`
	State     State      `json:"state"`
	Status    string     `json:"status,omitempty"`
	Health    string     `json:"health,omitempty"`
	ExitCode  *int       `json:"exitCode,omitempty"`
	Stats     *Stats     `json:"stats,omitempty"`
	Operation *Operation `json:"operation,omitempty"`
	DNS       *DNS       `json:"dns,omitempty"`

	External bool   `json:"external,omitempty"`
	Project  string `json:"project,omitempty"`
	Service  string `json:"service,omitempty"`
}

type Operation struct {
	Kind      string    `json:"kind"`
	Code      string    `json:"code,omitempty"`
	Message   string    `json:"message"`
	Percent   *int      `json:"percent,omitempty"`
	StartedAt time.Time `json:"startedAt"`
	Error     string    `json:"error,omitempty"`
}

var nameRE = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{1,38}$`)

func ValidateName(name string) error {
	if !nameRE.MatchString(name) {
		return fmt.Errorf("nome inválido: use 2 a 39 caracteres entre minúsculas, dígitos, - e _, começando por letra ou dígito")
	}
	return nil
}

func ParseMemory(s string) (int64, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return 0, nil
	}
	mult := int64(1)
	switch {
	case strings.HasSuffix(s, "g"), strings.HasSuffix(s, "gb"):
		mult = 1 << 30
	case strings.HasSuffix(s, "m"), strings.HasSuffix(s, "mb"):
		mult = 1 << 20
	case strings.HasSuffix(s, "k"), strings.HasSuffix(s, "kb"):
		mult = 1 << 10
	}
	digits := strings.TrimRight(s, "gmkb")
	n, err := strconv.ParseFloat(digits, 64)
	if err != nil {
		return 0, fmt.Errorf("limite de memória inválido: %q", s)
	}
	if n <= 0 {
		return 0, fmt.Errorf("limite de memória precisa ser positivo: %q", s)
	}
	return int64(n * float64(mult)), nil
}

func FormatMemory(b int64) string {
	switch {
	case b >= 1<<30 && b%(1<<30) == 0:
		return strconv.FormatInt(b/(1<<30), 10) + "g"
	case b >= 1<<20 && b%(1<<20) == 0:
		return strconv.FormatInt(b/(1<<20), 10) + "m"
	default:
		return strconv.FormatInt(b, 10)
	}
}
