// Command shugoshin is the CLI entry point for the Shugoshin code-review tool.
// Run without arguments to launch the interactive TUI. Sub-commands manage
// project initialisation and are wired into Claude Code hook events.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/zeeshans/shugoshin/internal/hooks"
	initpkg "github.com/zeeshans/shugoshin/internal/init"
	"github.com/zeeshans/shugoshin/internal/logger"
	"github.com/zeeshans/shugoshin/internal/reports"
	"github.com/zeeshans/shugoshin/internal/tui"
)

func main() {
	if err := rootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

func rootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "shugoshin",
		Short: "Shugoshin — autonomous code review guard for Claude Code",
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("getting working directory: %w", err)
			}
			baseDir := filepath.Join(cwd, ".shugoshin")
			return tui.Run(baseDir, reports.ListReports)
		},
	}

	root.AddCommand(
		initCmd(),
		deinitCmd(),
		cleanupCmd(),
		hookCmd(),
	)

	return root
}

func initCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Initialise Shugoshin in the current project",
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("getting working directory: %w", err)
			}
			return initpkg.Init(cwd)
		},
	}
}

func deinitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "deinit",
		Short: "Remove Shugoshin from the current project",
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("getting working directory: %w", err)
			}
			return initpkg.Deinit(cwd)
		},
	}
}

func cleanupCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "cleanup",
		Short: "Clear state and reports, keeping hook configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("getting working directory: %w", err)
			}
			return initpkg.Cleanup(cwd)
		},
	}
}

// hookCmd returns the parent "hook" command and its sub-commands.
// Hook sub-commands always exit 0 to avoid crashing Claude Code; errors are
// printed to stderr.
func hookCmd() *cobra.Command {
	hook := &cobra.Command{
		Use:   "hook",
		Short: "Claude Code hook sub-commands (internal use)",
	}

	hook.AddCommand(hookSubmitCmd(), hookPostToolCmd(), hookStopCmd(), hookAnalyseCmd())
	return hook
}

func hookSubmitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "submit",
		Short: "Handle UserPromptSubmit hook",
		Run: func(cmd *cobra.Command, args []string) {
			defer logger.Close()
			if err := hooks.HandleSubmit(os.Stdin); err != nil {
				fmt.Fprintf(os.Stderr, "shugoshin hook submit: %v\n", err)
			}
			os.Exit(0)
		},
	}
}

func hookPostToolCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "posttool",
		Short: "Handle PostToolUse hook",
		Run: func(cmd *cobra.Command, args []string) {
			defer logger.Close()
			if err := hooks.HandlePostTool(os.Stdin); err != nil {
				fmt.Fprintf(os.Stderr, "shugoshin hook posttool: %v\n", err)
			}
			os.Exit(0)
		},
	}
}

func hookStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Handle Stop hook",
		Run: func(cmd *cobra.Command, args []string) {
			defer logger.Close()
			if err := hooks.HandleStop(os.Stdin); err != nil {
				fmt.Fprintf(os.Stderr, "shugoshin hook stop: %v\n", err)
			}
			os.Exit(0)
		},
	}
}

func hookAnalyseCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "analyse [request-file]",
		Short:  "Run background analysis (internal use)",
		Args:   cobra.ExactArgs(1),
		Hidden: true,
		Run: func(cmd *cobra.Command, args []string) {
			defer logger.Close()
			if err := hooks.HandleAnalyse(args[0], nil); err != nil {
				fmt.Fprintf(os.Stderr, "shugoshin hook analyse: %v\n", err)
			}
			os.Exit(0)
		},
	}
}
