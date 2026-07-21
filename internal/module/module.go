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

// knownProviders maps a provider short name to its registry source namespace.
var knownProviders = map[string]string{
	"aws":        "hashicorp/aws",
	"google":     "hashicorp/google",
	"azurerm":    "hashicorp/azurerm",
	"kubernetes": "hashicorp/kubernetes",
	"null":       "hashicorp/null",
}

// ProviderSource returns the registry source for the spec's provider.
func (s Spec) ProviderSource() string {
	if src, ok := knownProviders[s.Provider]; ok {
		return src
	}
	return "hashicorp/" + s.Provider
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
		s.ProviderVersion = ">= 5.0.0"
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
	}
	return nil
}
