package catalog

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
)

type FieldType string

const (
	FieldText     FieldType = "text"
	FieldPassword FieldType = "password"
	FieldInt      FieldType = "int"
	FieldFloat    FieldType = "float"
	FieldBool     FieldType = "bool"
	FieldEnum     FieldType = "enum"
)

type FieldTarget string

const (
	TargetEnv FieldTarget = "env"
	TargetArg FieldTarget = "arg"
)

type Option struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

type Field struct {
	Key      string      `json:"key"`
	Label    string      `json:"label"`
	Type     FieldType   `json:"type"`
	Default  string      `json:"default,omitempty"`
	Required bool        `json:"required,omitempty"`
	Secret   bool        `json:"secret,omitempty"`
	Min      *float64    `json:"min,omitempty"`
	Max      *float64    `json:"max,omitempty"`
	Options  []Option    `json:"options,omitempty"`
	Help     string      `json:"help,omitempty"`
	Advanced bool        `json:"advanced,omitempty"`
	Target   FieldTarget `json:"target,omitempty"`
	Flag     string      `json:"flag,omitempty"`
}

func (f Field) IsArg() bool { return f.Target == TargetArg }

type Port struct {
	Container   int    `json:"container"`
	Protocol    string `json:"protocol"`
	DefaultHost int    `json:"defaultHost"`
	Label       string `json:"label"`
	Optional    bool   `json:"optional,omitempty"`
}

type Volume struct {
	Host      string `json:"host"`
	Container string `json:"container"`
	Data      bool   `json:"data,omitempty"`
}

type Provider struct {
	ID               string   `json:"id"`
	Game             string   `json:"game"`
	GameLabel        string   `json:"gameLabel"`
	Short            string   `json:"short"`
	Image            string   `json:"image"`
	Description      string   `json:"description"`
	Docs             string   `json:"docs,omitempty"`
	Ports            []Port   `json:"ports"`
	Volumes          []Volume `json:"volumes"`
	DefaultMemory    string   `json:"defaultMemory"`
	MinMemory        string   `json:"minMemory"`
	DefaultCPUs      float64  `json:"defaultCpus"`
	Tags             []string `json:"tags,omitempty"`
	ImagePattern     string   `json:"imagePattern,omitempty"`
	StopGraceSeconds int      `json:"stopGraceSeconds"`
	Fields           []Field  `json:"fields"`
}

func (p Provider) Field(key string) (Field, bool) {
	for _, f := range p.Fields {
		if f.Key == key {
			return f, true
		}
	}
	return Field{}, false
}

func (p Provider) DataVolume() (Volume, bool) {
	for _, v := range p.Volumes {
		if v.Data {
			return v, true
		}
	}
	return Volume{}, false
}

func (p Provider) Defaults() map[string]string {
	out := make(map[string]string, len(p.Fields))
	for _, f := range p.Fields {
		if f.Default != "" {
			out[f.Key] = f.Default
		}
	}
	return out
}

func (p Provider) Validate(values map[string]string) (map[string]string, error) {
	out := p.Defaults()
	var problems []Problem

	known := make(map[string]Field, len(p.Fields))
	for _, f := range p.Fields {
		known[f.Key] = f
	}

	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		v := values[k]
		f, ok := known[k]
		if !ok {
			if p.acceptsFreeEnv() {
				out[k] = v
				continue
			}
			problems = append(problems, Problem{
				Field:  k,
				Code:   "unknown_field",
				Params: map[string]any{"provider": p.ID},
			})
			continue
		}
		norm, bad := validateField(f, v)
		if bad != nil {
			problems = append(problems, Problem{Field: k, Code: bad.Code, Params: bad.Params})
			continue
		}
		out[k] = norm
	}

	for _, f := range p.Fields {
		if f.Required && strings.TrimSpace(out[f.Key]) == "" {
			problems = append(problems, Problem{Field: f.Key, Code: "required"})
		}
	}

	if len(problems) > 0 {
		return nil, &ValidationError{Problems: problems}
	}
	return out, nil
}

func (p Provider) acceptsFreeEnv() bool { return p.ID == CustomProviderID }

func validateField(f Field, v string) (string, *Problem) {
	v = strings.TrimSpace(v)
	if v == "" {
		return "", nil
	}
	switch f.Type {
	case FieldInt:
		n, err := strconv.Atoi(v)
		if err != nil {
			return "", &Problem{Code: "not_int", Params: map[string]any{"value": v}}
		}
		if f.Min != nil && float64(n) < *f.Min {
			return "", &Problem{Code: "below_min", Params: map[string]any{"min": *f.Min}}
		}
		if f.Max != nil && float64(n) > *f.Max {
			return "", &Problem{Code: "above_max", Params: map[string]any{"max": *f.Max}}
		}
		return strconv.Itoa(n), nil
	case FieldFloat:
		n, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return "", &Problem{Code: "not_number", Params: map[string]any{"value": v}}
		}
		if f.Min != nil && n < *f.Min {
			return "", &Problem{Code: "below_min", Params: map[string]any{"min": *f.Min}}
		}
		if f.Max != nil && n > *f.Max {
			return "", &Problem{Code: "above_max", Params: map[string]any{"max": *f.Max}}
		}
		return strconv.FormatFloat(n, 'f', -1, 64), nil
	case FieldBool:
		b, err := strconv.ParseBool(strings.ToLower(v))
		if err != nil {
			return "", &Problem{Code: "not_bool", Params: map[string]any{"value": v}}
		}
		return strconv.FormatBool(b), nil
	case FieldEnum:
		for _, o := range f.Options {
			if o.Value == v {
				return v, nil
			}
		}
		allowed := make([]string, len(f.Options))
		for i, o := range f.Options {
			allowed[i] = o.Value
		}
		return "", &Problem{
			Code:   "not_option",
			Params: map[string]any{"value": v, "allowed": strings.Join(allowed, ", ")},
		}
	default:
		return v, nil
	}
}

var imagePatterns sync.Map

func (p Provider) AcceptsImage(image string) bool {
	if p.ImagePattern == "" {
		return true
	}
	re, ok := imagePatterns.Load(p.ImagePattern)
	if !ok {
		compiled, err := regexp.Compile(p.ImagePattern)
		if err != nil {
			return true
		}
		imagePatterns.Store(p.ImagePattern, compiled)
		re = compiled
	}
	return re.(*regexp.Regexp).MatchString(image)
}

func ProviderForImage(image string) (Provider, bool) {
	for _, p := range providers {
		if p.ID == CustomProviderID || p.ImagePattern == "" {
			continue
		}
		if p.AcceptsImage(image) {
			return p, true
		}
	}
	return Provider{}, false
}

// Problem e campo reprovado: code e params bastam para a tela escrever a frase
type Problem struct {
	Field  string         `json:"field"`
	Code   string         `json:"code"`
	Params map[string]any `json:"params,omitempty"`
}

type ValidationError struct {
	Problems []Problem
}

func (e *ValidationError) Error() string {
	parts := make([]string, len(e.Problems))
	for i, p := range e.Problems {
		parts[i] = fmt.Sprintf("%s: %s", p.Field, p.Code)
	}
	return strings.Join(parts, "; ")
}

func (p Provider) SplitValues(values map[string]string) (env map[string]string, args []string) {
	env = make(map[string]string, len(values))
	isArg := make(map[string]Field, len(p.Fields))
	for _, f := range p.Fields {
		if f.IsArg() {
			isArg[f.Key] = f
		}
	}

	for k, v := range values {
		f, ok := isArg[k]
		if !ok || f.Secret {
			env[k] = v
		}
	}

	for _, f := range p.Fields {
		if !f.IsArg() {
			continue
		}
		v := strings.TrimSpace(values[f.Key])
		if v == "" {
			continue
		}
		if f.Type == FieldBool {
			if v == "true" {
				args = append(args, f.Flag)
			}
			continue
		}
		if f.Secret {
			args = append(args, f.Flag, "${"+f.Key+"}")
			continue
		}
		args = append(args, f.Flag, v)
	}
	return env, args
}
