package cmd

import (
	"bufio"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/moveeeax/terraform-module-scaffold/internal/module"
	"github.com/moveeeax/terraform-module-scaffold/internal/scaffold"
	"github.com/spf13/cobra"
)

func newNewCmd() *cobra.Command {
	var (
		name        string
		provider    string
		description string
		dest        string
		configPath  string
		force       bool
		noInput     bool
	)

	cmd := &cobra.Command{
		Use:   "new",
		Short: "Generate a new Terraform module skeleton",
		Example: "  terraform-module-scaffold new --name vpc --provider aws\n" +
			"  terraform-module-scaffold new --config module.yaml -o ./modules/vpc",
		RunE: func(cmd *cobra.Command, _ []string) error {
			var spec module.Spec
			if configPath != "" {
				loaded, err := module.LoadConfig(configPath)
				if err != nil {
					return err
				}
				spec = loaded
			}
			if name != "" {
				spec.Name = name
			}
			if provider != "" {
				spec.Provider = provider
			}
			if description != "" {
				spec.Description = description
			}

			if configPath == "" && !noInput {
				in := bufio.NewReader(cmd.InOrStdin())
				if spec.Name == "" {
					spec.Name = prompt(cmd, in, "Module name", "")
				}
				if spec.Provider == "" {
					spec.Provider = prompt(cmd, in, "Provider", "aws")
				}
				if spec.Description == "" {
					spec.Description = prompt(cmd, in, "Description", "Terraform module "+spec.Name)
				}
			}
			if spec.Provider == "" {
				spec.Provider = "aws"
			}
			if dest == "" {
				dest = spec.Name
			}

			// The scaffold only has verified registry metadata for the
			// providers it knows. For anything else both the source address and
			// the version constraint in versions.tf are guesses, and a wrong
			// guess fails at `terraform init`, not here — so say so.
			if !spec.KnownProvider() {
				fmt.Fprintf(cmd.ErrOrStderr(),
					"warning: provider %q is not one this scaffold knows; check versions.tf — source = %q and the version constraint are guesses\n",
					spec.Provider, spec.ProviderSource())
			}

			written, err := scaffold.Generate(spec, scaffold.Options{Dest: dest, Force: force})
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Scaffolded module %q (%s) into %s:\n", spec.Name, spec.Provider, dest)
			for _, f := range written {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", filepath.Join(dest, f))
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "module name")
	cmd.Flags().StringVar(&provider, "provider", "", "primary provider (aws, google, azurerm, kubernetes, null, ...)")
	cmd.Flags().StringVar(&description, "description", "", "module description")
	cmd.Flags().StringVarP(&dest, "output-dir", "o", "", "output directory (default: module name)")
	cmd.Flags().StringVarP(&configPath, "config", "c", "", "YAML config for non-interactive generation")
	cmd.Flags().BoolVar(&force, "force", false, "overwrite a non-empty destination")
	cmd.Flags().BoolVar(&noInput, "no-input", false, "never prompt; use flags/config and defaults only")
	return cmd
}

func prompt(cmd *cobra.Command, in *bufio.Reader, label, def string) string {
	if def != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "%s [%s]: ", label, def)
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "%s: ", label)
	}
	line, _ := in.ReadString('\n')
	if line = strings.TrimSpace(line); line == "" {
		return def
	}
	return line
}
