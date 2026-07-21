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
