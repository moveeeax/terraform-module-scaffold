// Package scaffold renders a Terraform module skeleton from embedded templates.
package scaffold

import (
	"bytes"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"text/template"

	"github.com/moveeeax/terraform-module-scaffold/internal/module"
)

//go:embed all:templates
var templatesFS embed.FS

type view struct {
	module.Spec
	HasNameVar bool
	HasTagsVar bool
	HasRuleset bool
}

// rulesetProviders have an official tflint ruleset plugin.
var rulesetProviders = map[string]bool{"aws": true, "google": true, "azurerm": true}

// ModuleArgs renders the argument lines of a module call block using the
// alignment `terraform fmt` enforces: within a run of arguments, every '='
// lines up on the longest attribute name.
func (v view) ModuleArgs(source string) string {
	type arg struct{ name, value string }
	args := []arg{{"source", strconv.Quote(source)}}
	for _, x := range v.Variables {
		if x.Required() {
			args = append(args, arg{x.Name, placeholder(x)})
		}
	}
	width := 0
	for _, a := range args {
		if len(a.name) > width {
			width = len(a.name)
		}
	}
	var b strings.Builder
	for i, a := range args {
		if i > 0 {
			b.WriteByte('\n')
		}
		fmt.Fprintf(&b, "  %-*s = %s", width, a.name, a.value)
	}
	return b.String()
}

func newView(s module.Spec) view {
	v := view{Spec: s, HasRuleset: rulesetProviders[s.Provider]}
	for _, x := range s.Variables {
		switch x.Name {
		case "name":
			v.HasNameVar = true
		case "tags":
			v.HasTagsVar = true
		}
	}
	return v
}

var funcs = template.FuncMap{"placeholder": placeholder}

// placeholder returns an example HCL literal for a variable's type, used in
// generated usage snippets.
func placeholder(v module.Variable) string {
	t := v.Type
	switch {
	case strings.HasPrefix(t, "list"), strings.HasPrefix(t, "set"), strings.HasPrefix(t, "tuple"):
		return "[]"
	case strings.HasPrefix(t, "map"), strings.HasPrefix(t, "object"):
		return "{}"
	case strings.Contains(t, "number"):
		return "0"
	case strings.Contains(t, "bool"):
		return "false"
	case strings.Contains(t, "string"):
		return `"..."`
	default:
		return "null"
	}
}

// Options controls generation.
type Options struct {
	Dest  string
	Force bool
}

// Generate renders the module described by spec into opts.Dest and returns the
// list of files written (relative to Dest).
func Generate(spec module.Spec, opts Options) ([]string, error) {
	spec.ApplyDefaults()
	if err := spec.Validate(); err != nil {
		return nil, err
	}
	if opts.Dest == "" {
		return nil, fmt.Errorf("destination is required")
	}
	if !opts.Force {
		if entries, err := os.ReadDir(opts.Dest); err == nil && len(entries) > 0 {
			return nil, fmt.Errorf("destination %s is not empty (use --force to overwrite)", opts.Dest)
		}
	}

	v := newView(spec)
	var written []string
	err := fs.WalkDir(templatesFS, "templates", func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		rel := strings.TrimSuffix(strings.TrimPrefix(p, "templates/"), ".tmpl")
		outPath := filepath.Join(opts.Dest, rel)

		raw, err := templatesFS.ReadFile(p)
		if err != nil {
			return err
		}
		tmpl, err := template.New(filepath.Base(p)).Funcs(funcs).Parse(string(raw))
		if err != nil {
			return fmt.Errorf("parse template %s: %w", p, err)
		}
		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, v); err != nil {
			return fmt.Errorf("render %s: %w", p, err)
		}
		if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(outPath, buf.Bytes(), 0o644); err != nil {
			return err
		}
		written = append(written, rel)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(written)
	return written, nil
}
