package scaffold

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/moveeeax/terraform-module-scaffold/internal/module"
)

func TestGenerateDefaults(t *testing.T) {
	dest := t.TempDir()
	written, err := Generate(module.Spec{Name: "vpc", Provider: "aws"}, Options{Dest: dest, Force: true})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	want := []string{
		"main.tf", "variables.tf", "outputs.tf", "versions.tf",
		"README.md", "Makefile", ".pre-commit-config.yaml", ".tflint.hcl",
		".gitignore", "examples/basic/main.tf", "examples/basic/README.md",
		"tests/defaults.tftest.hcl", ".github/workflows/ci.yml",
		"test/module_test.go", "test/go.mod",
	}
	for _, w := range want {
		if !contains(written, w) {
			t.Errorf("expected %s in generated list %v", w, written)
		}
		if _, err := os.Stat(filepath.Join(dest, w)); err != nil {
			t.Errorf("file %s missing on disk: %v", w, err)
		}
	}

	versions := read(t, dest, "versions.tf")
	if !strings.Contains(versions, "hashicorp/aws") {
		t.Errorf("versions.tf missing provider source:\n%s", versions)
	}

	vars := read(t, dest, "variables.tf")
	if !strings.Contains(vars, `variable "name"`) || !strings.Contains(vars, "type        = map(string)") {
		t.Errorf("variables.tf unexpected:\n%s", vars)
	}
	if !strings.Contains(vars, "default     = {}") {
		t.Errorf("variables.tf missing tags default:\n%s", vars)
	}

	readme := read(t, dest, "README.md")
	for _, sub := range []string{"# vpc", "## Inputs", "## Outputs", "`name`", "`tags`", "| terraform |"} {
		if !strings.Contains(readme, sub) {
			t.Errorf("README missing %q", sub)
		}
	}

	main := read(t, dest, "main.tf")
	if !strings.Contains(main, "name = var.name") || !strings.Contains(main, "var.tags") {
		t.Errorf("main.tf missing locals wiring:\n%s", main)
	}

	tflint := read(t, dest, ".tflint.hcl")
	if !strings.Contains(tflint, `plugin "aws"`) {
		t.Errorf(".tflint.hcl should include aws ruleset:\n%s", tflint)
	}
	if !strings.Contains(tflint, `version = "`+rulesetVersions["aws"]+`"`) {
		t.Errorf(".tflint.hcl should pin the aws ruleset version:\n%s", tflint)
	}
}

// Provider version floors that the registry cannot satisfy were the scaffold's
// worst failure mode: the generated module died at `terraform init`. Assert the
// per-provider floor reaches versions.tf, and that nothing reintroduces a
// blanket ">= 5.0.0".
func TestGenerateProviderVersionFloor(t *testing.T) {
	for provider, want := range map[string]string{
		"aws":        ">= 6.0",
		"azurerm":    ">= 4.0",
		"kubernetes": ">= 2.30",
		"null":       ">= 3.0",
	} {
		dest := t.TempDir()
		if _, err := Generate(module.Spec{Name: "m", Provider: provider}, Options{Dest: dest, Force: true}); err != nil {
			t.Fatalf("Generate(%s): %v", provider, err)
		}
		versions := read(t, dest, "versions.tf")
		if !strings.Contains(versions, `version = "`+want+`"`) {
			t.Errorf("%s versions.tf should pin %q:\n%s", provider, want, versions)
		}
	}
}

// The generated module must ship a runnable `terraform test` suite and a
// workflow that runs it — the two things every module generated before this
// was missing.
func TestGenerateTestSuiteAndWorkflow(t *testing.T) {
	dest := t.TempDir()
	if _, err := Generate(module.Spec{Name: "vpc", Provider: "aws"}, Options{Dest: dest, Force: true}); err != nil {
		t.Fatalf("Generate: %v", err)
	}

	suite := read(t, dest, "tests/defaults.tftest.hcl")
	for _, sub := range []string{
		`mock_provider "aws" {}`,
		`run "plans_with_required_inputs_only"`,
		`name = "..."`,
		`output.tags["Module"] == "vpc"`,
		"Terraform >= 1.7",
	} {
		if !strings.Contains(suite, sub) {
			t.Errorf("tests/defaults.tftest.hcl missing %q:\n%s", sub, suite)
		}
	}

	// The test-only 1.7 floor must not leak into what consumers have to run.
	versions := read(t, dest, "versions.tf")
	if !strings.Contains(versions, `required_version = ">= 1.5.0"`) {
		t.Errorf("required_version should stay at the module's own floor:\n%s", versions)
	}

	ci := read(t, dest, ".github/workflows/ci.yml")
	for _, sub := range []string{
		"permissions:\n  contents: read",
		"terraform test",
		"${{ secrets.GITHUB_TOKEN }}",
	} {
		if !strings.Contains(ci, sub) {
			t.Errorf("generated workflow missing %q:\n%s", sub, ci)
		}
	}
}

// A module whose outputs do not expose local.tags cannot assert on the merge,
// so the suite falls back to a plan-only run rather than emitting an assertion
// that is guaranteed to fail.
func TestGenerateTestSuiteWithoutTagsOutput(t *testing.T) {
	dest := t.TempDir()
	spec := module.Spec{
		Name:      "bucket",
		Provider:  "aws",
		Variables: []module.Variable{{Name: "bucket_name", Type: "string", Description: "Name"}},
		Outputs:   []module.Output{{Name: "name", Description: "Name", Value: "var.bucket_name"}},
	}
	if _, err := Generate(spec, Options{Dest: dest, Force: true}); err != nil {
		t.Fatalf("Generate: %v", err)
	}
	suite := read(t, dest, "tests/defaults.tftest.hcl")
	if strings.Contains(suite, "module_tags_merge_with_caller_tags") {
		t.Errorf("tag assertions require a tags variable and a local.tags output:\n%s", suite)
	}
	if !strings.Contains(suite, `bucket_name = "..."`) {
		t.Errorf("required inputs should still be wired:\n%s", suite)
	}
}

// Descriptions are free-form text from a user-supplied spec. Pasting them into
// HCL unescaped produced files Terraform could not parse.
func TestGenerateEscapesDescriptions(t *testing.T) {
	dest := t.TempDir()
	spec := module.Spec{
		Name:     "quoting",
		Provider: "null",
		Variables: []module.Variable{
			{Name: "region", Type: "string", Description: `Region such as "us-east-1", costs ${price}/hr.`},
		},
		Outputs: []module.Output{
			{Name: "region", Description: `Echoes "region" back | verbatim.`, Value: "var.region"},
		},
	}
	if _, err := Generate(spec, Options{Dest: dest, Force: true}); err != nil {
		t.Fatalf("Generate: %v", err)
	}

	vars := read(t, dest, "variables.tf")
	if !strings.Contains(vars, `description = "Region such as \"us-east-1\", costs $${price}/hr."`) {
		t.Errorf("variables.tf description not escaped:\n%s", vars)
	}
	outputs := read(t, dest, "outputs.tf")
	if !strings.Contains(outputs, `\"region\"`) {
		t.Errorf("outputs.tf description not escaped:\n%s", outputs)
	}

	// A pipe in a description would otherwise break the Markdown table.
	readme := read(t, dest, "README.md")
	if !strings.Contains(readme, `Echoes "region" back \| verbatim.`) {
		t.Errorf("README table cell not escaped:\n%s", readme)
	}
}

func TestGenerateProviderWithoutRuleset(t *testing.T) {
	dest := t.TempDir()
	if _, err := Generate(module.Spec{Name: "noop", Provider: "null"}, Options{Dest: dest, Force: true}); err != nil {
		t.Fatalf("Generate: %v", err)
	}
	tflint := read(t, dest, ".tflint.hcl")
	if strings.Contains(tflint, `plugin "null"`) {
		t.Errorf("null provider should not get a ruleset plugin:\n%s", tflint)
	}
	versions := read(t, dest, "versions.tf")
	if !strings.Contains(versions, "hashicorp/null") {
		t.Errorf("versions.tf wrong source:\n%s", versions)
	}
}

func TestGenerateCustomVariablesTable(t *testing.T) {
	dest := t.TempDir()
	spec := module.Spec{
		Name:     "bucket",
		Provider: "aws",
		Variables: []module.Variable{
			{Name: "bucket_name", Type: "string", Description: "Bucket name"},
			{Name: "versioning", Type: "bool", Description: "Enable versioning", Default: "true"},
		},
		Outputs: []module.Output{
			{Name: "arn", Description: "Bucket ARN", Value: "aws_s3_bucket.this.arn"},
		},
	}
	if _, err := Generate(spec, Options{Dest: dest, Force: true}); err != nil {
		t.Fatalf("Generate: %v", err)
	}
	readme := read(t, dest, "README.md")
	if !strings.Contains(readme, "| `bucket_name` |") || !strings.Contains(readme, "| yes |") {
		t.Errorf("README inputs table wrong:\n%s", readme)
	}
	example := read(t, dest, "examples/basic/main.tf")
	if !strings.Contains(example, `bucket_name = "..."`) {
		t.Errorf("example should wire required string var:\n%s", example)
	}
}

// Generated HCL must already satisfy `terraform fmt`, which aligns the '=' of
// consecutive arguments on the longest attribute name.
func TestModuleArgsAlignment(t *testing.T) {
	spec := module.Spec{
		Name:     "bucket",
		Provider: "aws",
		Variables: []module.Variable{
			{Name: "bucket_name", Type: "string", Description: "Bucket name"},
			{Name: "az", Type: "string", Description: "AZ"},
			{Name: "tags", Type: "map(string)", Description: "Tags", Default: "{}"},
		},
	}
	spec.ApplyDefaults()
	got := newView(spec).ModuleArgs("../../")
	want := "  source      = \"../../\"\n" +
		"  az          = \"...\"\n" +
		"  bucket_name = \"...\""
	if got != want {
		t.Errorf("ModuleArgs alignment:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestGenerateRefusesNonEmpty(t *testing.T) {
	dest := t.TempDir()
	if err := os.WriteFile(filepath.Join(dest, "keep"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Generate(module.Spec{Name: "a", Provider: "aws"}, Options{Dest: dest}); err == nil {
		t.Fatal("expected error for non-empty destination without --force")
	}
}

func TestGenerateInvalidName(t *testing.T) {
	if _, err := Generate(module.Spec{Name: "9bad", Provider: "aws"}, Options{Dest: t.TempDir(), Force: true}); err == nil {
		t.Fatal("expected validation error for bad name")
	}
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

func read(t *testing.T, dir, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(b)
}
