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
	// RulesetVersion is the tflint ruleset plugin version for Provider.
	RulesetVersion string
	// CanAssertTags is true when the module both accepts a `tags` variable and
	// exposes the merged result as `output "tags"`, which is what lets the
	// generated test suite assert on the merge instead of only planning.
	CanAssertTags bool
}

// rulesetVersions maps a provider to the version of its official tflint ruleset
// plugin. `tflint --init` fails outright on a version that was never released,
// so these are real tags from github.com/terraform-linters.
var rulesetVersions = map[string]string{
	"aws":     "0.48.0",
	"google":  "0.39.0",
	"azurerm": "0.32.0",
}

type arg struct{ name, value string }

// assignments renders `name = value` lines indented by indent spaces, using the
// alignment `terraform fmt` enforces: within a run of arguments, every '=' lines
// up on the longest attribute name.
func assignments(indent int, args []arg) string {
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
		fmt.Fprintf(&b, "%s%-*s = %s", strings.Repeat(" ", indent), width, a.name, a.value)
	}
	return b.String()
}

// requiredArgs is one placeholder assignment per variable that has no default.
func (v view) requiredArgs() []arg {
	var args []arg
	for _, x := range v.Variables {
		if x.Required() {
			args = append(args, arg{x.Name, placeholder(x)})
		}
	}
	return args
}

// ModuleArgs renders the argument lines of a module call block.
func (v view) ModuleArgs(source string) string {
	return assignments(2, append([]arg{{"source", strconv.Quote(source)}}, v.requiredArgs()...))
}

// RequiredVarAssignments renders a `variables { ... }` body wiring every
// required input to a placeholder. Empty when the module has no required
// inputs, so callers can drop the block entirely.
func (v view) RequiredVarAssignments(indent int) string {
	return assignments(indent, v.requiredArgs())
}

// TagTestAssignments renders a `variables { ... }` body that also passes a
// caller-supplied tag, for the generated test that asserts the tag merge.
func (v view) TagTestAssignments(indent int) string {
	args := make([]arg, 0, len(v.Variables)+1)
	for _, a := range v.requiredArgs() {
		if a.name == "tags" {
			continue
		}
		args = append(args, a)
	}
	return assignments(indent, append(args, arg{"tags", `{ Owner = "platform-team" }`}))
}

func newView(s module.Spec) view {
	v := view{Spec: s}
	v.RulesetVersion, v.HasRuleset = rulesetVersions[s.Provider]
	for _, x := range s.Variables {
		switch x.Name {
		case "name":
			v.HasNameVar = true
		case "tags":
			v.HasTagsVar = true
		}
	}
	for _, o := range s.Outputs {
		if o.Name == "tags" && strings.TrimSpace(o.Value) == "local.tags" {
			v.CanAssertTags = v.HasTagsVar
		}
	}
	return v
}

var funcs = template.FuncMap{
	"placeholder": placeholder,
	"hcl":         hclString,
	"md":          mdCell,
}

// hclString renders s as a quoted HCL string. Descriptions come from a
// user-supplied YAML spec, so a stray quote, backslash or `${...}` sequence
// would otherwise be pasted straight into the module and make it unparseable.
func hclString(s string) string {
	q := strconv.Quote(s)
	q = strings.ReplaceAll(q, "${", "$${")
	q = strings.ReplaceAll(q, "%{", "%%{")
	return q
}

// mdCell escapes text for a Markdown table cell: a pipe or newline in a
// description would otherwise shred the generated inputs/outputs tables.
func mdCell(s string) string {
	s = strings.ReplaceAll(s, "|", `\|`)
	s = strings.ReplaceAll(s, "\r\n", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	return strings.TrimSpace(s)
}

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
