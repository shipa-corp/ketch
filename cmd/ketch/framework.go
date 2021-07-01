package main

import (
	"io"

	"github.com/spf13/cobra"
)

const frameworkHelp = `
Manage frameworks.

NOTE: "pool" has been deprecated and replaced with "framework". The functionality is the same.
`

func newFrameworkCmd(cfg config, out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "framework",
		Aliases: []string{"pool"},
		Short:   "Manage frameworks",
		Long:    frameworkHelp,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Usage()
		},
	}
	cmd.AddCommand(newFrameworkListCmd(cfg, out))
	cmd.AddCommand(newFrameworkAddCmd(cfg, out, addFramework))
	cmd.AddCommand(newFrameworkRemoveCmd(cfg, out))
	cmd.AddCommand(newFrameworkUpdateCmd(cfg, out))
	cmd.AddCommand(newFrameworkExportCmd(cfg))
	return cmd
}
