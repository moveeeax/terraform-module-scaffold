// Package cmd wires the terraform-module-scaffold CLI.
package cmd

import "github.com/spf13/cobra"

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:          "terraform-module-scaffold",
		Short:        "Scaffold a production-grade Terraform module in seconds",
		Long:         "terraform-module-scaffold generates a best-practice Terraform module skeleton\n(main/variables/outputs/versions, examples, Terratest, pre-commit, tflint, README).",
		SilenceUsage: true,
	}
	root.AddCommand(newNewCmd())
	return root
}

// Execute runs the root command.
func Execute() error {
	return newRootCmd().Execute()
}
