// Package module defines the specification of a Terraform module to scaffold.
package module

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Variable is a single Terraform input variable.
type Variable struct {
	Name        string `yaml:"name"`
	Type        string `yaml:"type"`
	Description string `yaml:"description"`
	Default     string `yaml:"default"`
	Sensitive   bool   `yaml:"sensitive"`
}

// Required reports whether the variable has no default and must be set.
func (v Variable) Required() bool { return strings.TrimSpace(v.Default) == "" }

// Output is a single Terraform output value.
type Output struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Value       string `yaml:"value"`
}

// Spec is the full description of a module to generate.
type Spec struct {
	Name             string     `yaml:"name"`
	Provider         string     `yaml:"provider"`
	Description      string     `yaml:"description"`
	TerraformVersion string     `yaml:"terraform_version"`
	ProviderVersion  string     `yaml:"provider_version"`
	Variables        []Variable `yaml:"variables"`
	Outputs          []Output   `yaml:"outputs"`
}

var nameRe = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)

// providerInfo is what a provider needs in a required_providers block.
type providerInfo struct {
	Source  string // registry source address
	Version string // default constraint, floored at the provider's current major
}

// knownProviders maps a provider short name to its registry source and a
// default version constraint.
//
// The constraint matters more than it looks: it is a floor the public registry
// has to be able to satisfy. A blanket ">= 5.0.0" is not a conservative default,
// it is a broken one — the kubernetes, null and azurerm providers have never
// published a 5.x, so a module scaffolded with it fails at `terraform init`
// before anyone gets to write a resource. Keep every floor here at a major that
// actually exists, and prefer the current major so a brand-new module starts on
// the provider generation it was written against.
var knownProviders = map[string]providerInfo{
	"archive":     {"hashicorp/archive", ">= 2.0"},
	"aws":         {"hashicorp/aws", ">= 6.0"},
	"azuread":     {"hashicorp/azuread", ">= 3.0"},
	"azurerm":     {"hashicorp/azurerm", ">= 4.0"},
	"cloudflare":  {"cloudflare/cloudflare", ">= 5.0"},
	"cloudinit":   {"hashicorp/cloudinit", ">= 2.0"},
	"datadog":     {"DataDog/datadog", ">= 3.0"},
	"external":    {"hashicorp/external", ">= 2.0"},
	"github":      {"integrations/github", ">= 6.0"},
	"google":      {"hashicorp/google", ">= 7.0"},
	"google-beta": {"hashicorp/google-beta", ">= 7.0"},
	"helm":        {"hashicorp/helm", ">= 3.0"},
	"http":        {"hashicorp/http", ">= 3.0"},
	"kubernetes":  {"hashicorp/kubernetes", ">= 2.30"},
	"local":       {"hashicorp/local", ">= 2.0"},
	"null":        {"hashicorp/null", ">= 3.0"},
	"random":      {"hashicorp/random", ">= 3.0"},
	"time":        {"hashicorp/time", ">= 0.9"},
	"tls":         {"hashicorp/tls", ">= 4.0"},
}

// fallbackProviderVersion is used for providers not in knownProviders. It is a
// guess, so `new` warns about it; ">= 1.0" at least resolves against the
// overwhelming majority of published providers.
const fallbackProviderVersion = ">= 1.0"

// KnownProvider reports whether the scaffold has verified source and version
// metadata for the spec's provider, or is guessing.
func (s Spec) KnownProvider() bool {
	_, ok := knownProviders[s.Provider]
	return ok
}

// ProviderSource returns the registry source for the spec's provider.
func (s Spec) ProviderSource() string {
	if p, ok := knownProviders[s.Provider]; ok {
		return p.Source
	}
	return "hashicorp/" + s.Provider
}

// DefaultProviderVersion returns the version constraint used when the spec does
// not pin one itself.
func (s Spec) DefaultProviderVersion() string {
	if p, ok := knownProviders[s.Provider]; ok {
		return p.Version
	}
	return fallbackProviderVersion
}

// ModuleName is a Terraform-friendly identifier derived from Name.
func (s Spec) ModuleName() string {
	return strings.ReplaceAll(s.Name, "-", "_")
}

// LoadConfig reads a Spec from a YAML file for non-interactive generation.
func LoadConfig(path string) (Spec, error) {
	var s Spec
	raw, err := os.ReadFile(path)
	if err != nil {
		return s, err
	}
	if err := yaml.Unmarshal(raw, &s); err != nil {
		return s, fmt.Errorf("parse %s: %w", path, err)
	}
	return s, nil
}

// ApplyDefaults fills empty fields with sane defaults so a bare
// "new --name x --provider aws" still yields a valid module.
func (s *Spec) ApplyDefaults() {
	if s.TerraformVersion == "" {
		s.TerraformVersion = ">= 1.5.0"
	}
	if s.ProviderVersion == "" {
		s.ProviderVersion = s.DefaultProviderVersion()
	}
	if s.Description == "" {
		s.Description = fmt.Sprintf("Terraform module %s", s.Name)
	}
	if len(s.Variables) == 0 {
		s.Variables = []Variable{
			{Name: "name", Type: "string", Description: "Name assigned to resources created by this module."},
			{Name: "tags", Type: "map(string)", Description: "Tags applied to all resources.", Default: "{}"},
		}
	}
	if len(s.Outputs) == 0 {
		s.Outputs = []Output{
			{Name: "name", Description: "Name propagated to resources.", Value: "local.name"},
			{Name: "tags", Description: "Effective tag set applied to resources.", Value: "local.tags"},
		}
	}
	// Deterministic ordering for reproducible output.
	sort.SliceStable(s.Variables, func(i, j int) bool { return s.Variables[i].Name < s.Variables[j].Name })
	sort.SliceStable(s.Outputs, func(i, j int) bool { return s.Outputs[i].Name < s.Outputs[j].Name })
}

// Validate checks that the spec is coherent enough to generate.
func (s Spec) Validate() error {
	if !nameRe.MatchString(s.Name) {
		return fmt.Errorf("invalid module name %q: use lowercase letters, digits, '-' or '_' and start with a letter", s.Name)
	}
	if strings.TrimSpace(s.Provider) == "" {
		return fmt.Errorf("provider is required")
	}
	if !nameRe.MatchString(s.Provider) {
		return fmt.Errorf("invalid provider %q", s.Provider)
	}
	seen := map[string]bool{}
	for _, v := range s.Variables {
		if !nameRe.MatchString(v.Name) {
			return fmt.Errorf("invalid variable name %q", v.Name)
		}
		if seen[v.Name] {
			return fmt.Errorf("duplicate variable %q", v.Name)
		}
		seen[v.Name] = true
		if strings.TrimSpace(v.Type) == "" {
			return fmt.Errorf("variable %q has no type", v.Name)
		}
	}
	seenOut := map[string]bool{}
	for _, o := range s.Outputs {
		if !nameRe.MatchString(o.Name) {
			return fmt.Errorf("invalid output name %q", o.Name)
		}
		if seenOut[o.Name] {
			return fmt.Errorf("duplicate output %q", o.Name)
		}
		seenOut[o.Name] = true
		// An output value is emitted verbatim as the right-hand side of
		// `value = `. Empty means unparseable HCL, so refuse here rather than
		// write a module that cannot be read back.
		if strings.TrimSpace(o.Value) == "" {
			return fmt.Errorf("output %q has no value: set `value` to the HCL expression the output should return", o.Name)
		}
	}
	return nil
}
