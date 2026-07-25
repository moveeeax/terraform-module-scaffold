package module

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestApplyDefaults(t *testing.T) {
	s := Spec{Name: "vpc", Provider: "aws"}
	s.ApplyDefaults()
	if len(s.Variables) != 2 {
		t.Fatalf("want 2 default variables, got %d", len(s.Variables))
	}
	if len(s.Outputs) != 2 {
		t.Fatalf("want 2 default outputs, got %d", len(s.Outputs))
	}
	if s.TerraformVersion == "" || s.ProviderVersion == "" {
		t.Fatal("versions not defaulted")
	}
	// Variables must be sorted.
	if s.Variables[0].Name != "name" || s.Variables[1].Name != "tags" {
		t.Fatalf("variables not sorted: %+v", s.Variables)
	}
}

func TestValidate(t *testing.T) {
	cases := []struct {
		desc string
		spec Spec
		ok   bool
	}{
		{"valid", Spec{Name: "vpc", Provider: "aws"}, true},
		{"bad name", Spec{Name: "1bad", Provider: "aws"}, false},
		{"no provider", Spec{Name: "vpc"}, false},
		{"underscore name", Spec{Name: "my_mod", Provider: "google"}, true},
	}
	for _, c := range cases {
		s := c.spec
		s.ApplyDefaults()
		err := s.Validate()
		if c.ok && err != nil {
			t.Errorf("%s: unexpected error: %v", c.desc, err)
		}
		if !c.ok && err == nil {
			t.Errorf("%s: expected error, got nil", c.desc)
		}
	}
}

func TestValidateDuplicateVariable(t *testing.T) {
	s := Spec{
		Name:     "vpc",
		Provider: "aws",
		Variables: []Variable{
			{Name: "a", Type: "string"},
			{Name: "a", Type: "string"},
		},
	}
	if err := s.Validate(); err == nil {
		t.Fatal("expected duplicate variable error")
	}
}

func TestRequired(t *testing.T) {
	if !(Variable{Default: ""}).Required() {
		t.Error("empty default should be required")
	}
	if (Variable{Default: "{}"}).Required() {
		t.Error("value default should be optional")
	}
}

func TestProviderSource(t *testing.T) {
	if got := (Spec{Provider: "aws"}).ProviderSource(); got != "hashicorp/aws" {
		t.Errorf("aws source = %q", got)
	}
	if got := (Spec{Provider: "custom"}).ProviderSource(); got != "hashicorp/custom" {
		t.Errorf("fallback source = %q", got)
	}
	// Not every provider lives under hashicorp/.
	if got := (Spec{Provider: "github"}).ProviderSource(); got != "integrations/github" {
		t.Errorf("github source = %q, want integrations/github", got)
	}
}

// The default provider version is a constraint the public registry has to be
// able to satisfy. A single blanket floor cannot be: kubernetes, null and
// azurerm have never published a 5.x, so ">= 5.0.0" made every module scaffolded
// for them fail at `terraform init`. The ci.yml "generated" job proves the
// floors resolve for real; this pins them so they cannot silently drift back.
func TestDefaultProviderVersionIsPerProvider(t *testing.T) {
	want := map[string]string{
		"aws":        ">= 6.0",
		"azurerm":    ">= 4.0",
		"google":     ">= 7.0",
		"kubernetes": ">= 2.30",
		"null":       ">= 3.0",
		"custom":     fallbackProviderVersion,
	}
	for provider, expect := range want {
		s := Spec{Name: "m", Provider: provider}
		s.ApplyDefaults()
		if s.ProviderVersion != expect {
			t.Errorf("%s default version = %q, want %q", provider, s.ProviderVersion, expect)
		}
	}

	// An explicit pin in the spec always wins.
	s := Spec{Name: "m", Provider: "aws", ProviderVersion: "~> 5.40"}
	s.ApplyDefaults()
	if s.ProviderVersion != "~> 5.40" {
		t.Errorf("explicit provider_version overwritten with %q", s.ProviderVersion)
	}
}

func TestKnownProvider(t *testing.T) {
	if !(Spec{Provider: "azurerm"}).KnownProvider() {
		t.Error("azurerm should be known")
	}
	if (Spec{Provider: "snowflake"}).KnownProvider() {
		t.Error("snowflake should not be known")
	}
}

func TestKnownProvidersAreComplete(t *testing.T) {
	for name, p := range knownProviders {
		if !strings.Contains(p.Source, "/") {
			t.Errorf("provider %q: source %q is not a registry address", name, p.Source)
		}
		if strings.TrimSpace(p.Version) == "" {
			t.Errorf("provider %q: empty version constraint", name)
		}
	}
}

// An output with no value renders `value = ` — unparseable HCL. Refuse it up
// front instead of writing a module that cannot be read back.
func TestValidateRejectsOutputWithoutValue(t *testing.T) {
	s := Spec{
		Name:     "vpc",
		Provider: "aws",
		Outputs:  []Output{{Name: "arn", Description: "ARN"}},
	}
	err := s.Validate()
	if err == nil {
		t.Fatal("expected error for output with no value")
	}
	if !strings.Contains(err.Error(), "arn") {
		t.Errorf("error should name the output: %v", err)
	}
}

func TestModuleName(t *testing.T) {
	if got := (Spec{Name: "my-vpc"}).ModuleName(); got != "my_vpc" {
		t.Errorf("ModuleName = %q, want my_vpc", got)
	}
}

func TestLoadConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "m.yaml")
	body := `name: bucket
provider: aws
description: An S3 bucket module
variables:
  - name: bucket_name
    type: string
    description: Name of the bucket
outputs:
  - name: arn
    description: Bucket ARN
    value: aws_s3_bucket.this.arn
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if s.Name != "bucket" || s.Provider != "aws" {
		t.Errorf("unexpected spec: %+v", s)
	}
	if len(s.Variables) != 1 || s.Variables[0].Name != "bucket_name" {
		t.Errorf("variables not parsed: %+v", s.Variables)
	}
	if len(s.Outputs) != 1 || s.Outputs[0].Name != "arn" {
		t.Errorf("outputs not parsed: %+v", s.Outputs)
	}
}
