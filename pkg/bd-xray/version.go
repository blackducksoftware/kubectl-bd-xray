package bd_xray

import (
	"fmt"

	"github.com/spf13/cobra"
)

// populated by goreleaser
var (
	semver string
	commit string
	date   string
)

func String() string {
	return fmt.Sprintf("Version: %s, Commit: %s, Build Date: %s", semver, commit, date)
}

func GetCurrent() string {
	return semver
}

func SetupVersionCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "version",
		Short: "Print version info",
		Long:  "Print version info",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(String())
		},
	}
	return command
}