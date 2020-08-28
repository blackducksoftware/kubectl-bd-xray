package bd_xray

import (
	"os"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/blackducksoftware/kubectl-bd-xray/pkg/util"
)

func InitAndExecute() {
	rootCmd := SetupRootCommand()
	if err := errors.Wrapf(rootCmd.Execute(), "run bd-xray root command"); err != nil {
		log.Fatalf("unable to run root command: %+v", err)
		os.Exit(1)
	}
}

type RootFlags struct {
	LogLevel string
	// GenericCliConfigFlags *genericclioptions.ConfigFlags
}

func SetupRootCommand() *cobra.Command {
	rootFlags := &RootFlags{}
	var rootCmd = &cobra.Command{
		Use:   "bd-xray",
		Short: "Run a Black Duck scan on an image",
		Long:  `Run a Black Duck scan on an image`,
		Args:  cobra.MaximumNArgs(0),
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return util.SetUpLogger(rootFlags.LogLevel)
		},
	}

	rootCmd.PersistentFlags().StringVarP(&rootFlags.LogLevel, "verbosity", "v", "info", "log level; one of [info, debug, trace, warn, error, fatal, panic]")

	// rootFlags.GenericCliConfigFlags = genericclioptions.NewConfigFlags(false)
	// rootFlags.GenericCliConfigFlags.AddFlags(rootCmd.Flags())

	rootCmd.AddCommand(SetupImageScanCommand())
	rootCmd.AddCommand(SetupNamespaceScanCommand())
	rootCmd.AddCommand(SetupYamlScanCommand())

	return rootCmd
}
