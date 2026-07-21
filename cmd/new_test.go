package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func run(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root := newRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(args)
	err := root.Execute()
	return out.String(), err
}

func TestNewWithFlags(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "vpc")
	out, err := run(t, "new", "--name", "vpc", "--provider", "aws", "--no-input", "-o", dest)
	if err != nil {
		t.Fatalf("execute: %v\n%s", err, out)
	}
	for _, f := range []string{"main.tf", "variables.tf", "outputs.tf", "versions.tf", "README.md"} {
		if _, err := os.Stat(filepath.Join(dest, f)); err != nil {
			t.Errorf("%s not generated: %v", f, err)
		}
	}
	if !strings.Contains(out, "Scaffolded module \"vpc\"") {
		t.Errorf("unexpected output:\n%s", out)
	}
}

func TestNewFromConfig(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "module.yaml")
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
	if err := os.WriteFile(cfg, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(dir, "out")
	if out, err := run(t, "new", "--config", cfg, "-o", dest); err != nil {
		t.Fatalf("execute: %v\n%s", err, out)
	}
	readme, err := os.ReadFile(filepath.Join(dest, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(readme), "`bucket_name`") {
		t.Errorf("README missing config-driven variable:\n%s", readme)
	}
}

func TestNewInteractivePrompts(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "mod")
	root := newRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetIn(strings.NewReader("cache\n\nA cache module\n"))
	root.SetArgs([]string{"new", "-o", dest})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, out.String())
	}
	readme, err := os.ReadFile(filepath.Join(dest, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	// Name came from stdin; provider fell back to the "aws" default on empty input.
	if !strings.Contains(string(readme), "# cache") || !strings.Contains(string(readme), "| aws |") {
		t.Errorf("prompts not applied:\n%s", readme)
	}
}

func TestNewRejectsBadName(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "x")
	if _, err := run(t, "new", "--name", "9bad", "--provider", "aws", "--no-input", "-o", dest); err == nil {
		t.Fatal("expected validation error")
	}
}
