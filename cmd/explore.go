package cmd

import (
	"github.com/alecthomas/repr"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"makexplorer/parser"
	"makexplorer/runner"
	"path/filepath"
)

func init() {
	rootCmd.AddCommand(explorerCmd)
}

var explorerCmd = &cobra.Command{
	Use:   "explore",
	Short: "Explore target",
	RunE: func(cmd *cobra.Command, args []string) error {
		filePath, err := filepath.Abs(args[0])
		if err != nil {
			return err
		}
		targetName := args[1]

		n, err := parser.ParseFile(filePath)
		if err != nil {
			return err
		}

		r := runner.New()
		r.RootDir = filepath.Dir(filePath)

		err = r.Include(n)
		if err != nil {
			return err
		}

		target, ok := r.Targets[targetName]
		if !ok {
			return errors.Errorf("unknown target")
		}

		repr.Println(target)

		return nil
	},
}
