package bd_xray

import (
	"os"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

func InitAndExecute() {
	rootCmd := setupRootCommand()
	if err := errors.Wrapf(rootCmd.Execute(), "run bd-xray root command"); err != nil {
		log.Fatalf("unable to run root command: %+v", err)
		os.Exit(1)
	}
}

type flagpole struct {
	logLevel              string
	genericCliConfigFlags *genericclioptions.ConfigFlags
}

func setupRootCommand() *cobra.Command {
	flags := &flagpole{}
	var rootCmd = &cobra.Command{
		Use:   "bd-xray",
		Short: "Run a blackduck scan on an image",
		Long:  `bd-xray`,
		Args:  cobra.MaximumNArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRootCommand(flags)
		},
	}

	rootCmd.PersistentFlags().StringVarP(&flags.logLevel, "verbosity", "v", "info", "log level; one of [info, debug, trace, warn, error, fatal, panic]")

	flags.genericCliConfigFlags = genericclioptions.NewConfigFlags(false)
	flags.genericCliConfigFlags.AddFlags(rootCmd.Flags())

	rootCmd.AddCommand(SetupImageScanCommand())
	return rootCmd
}

func runRootCommand(flags *flagpole) error {

	logLevel, _ := log.ParseLevel(flags.logLevel)
	log.SetLevel(logLevel)
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp: true,
	})

	return nil
}
