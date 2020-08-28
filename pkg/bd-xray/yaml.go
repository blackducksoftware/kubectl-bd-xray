package bd_xray

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/blackducksoftware/kubectl-bd-xray/pkg/util"
	"github.com/blackducksoftware/kubectl-bd-xray/pkg/yaml"
)

type yamlScanFlags struct {
	DetectOfflineMode string
	BlackDuckURL      string
	BlackDuckToken    string
	DetectProjectName string
	// TODO: add how many scans to process simultaneously
	// ConcurrencyLevel  string
}

func SetupNamespaceScanCommand() *cobra.Command {
	yamlScanFlags := &yamlScanFlags{}

	detectPassThroughFlagsMap := map[string]interface{}{
		DetectOfflineModeFlag: &yamlScanFlags.DetectOfflineMode,
		BlackDuckURLFlag:      &yamlScanFlags.BlackDuckURL,
		BlackDuckTokenFlag:    &yamlScanFlags.BlackDuckToken,
	}

	ctx, cancel := context.WithCancel(context.Background())

	command := &cobra.Command{
		Use:   "yaml YAML_FILE...",
		Short: "scan all yaml files provided",
		Long:  "scan all yaml files provided",
		Args: func(cmd *cobra.Command, args []string) error {
			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			util.DoOrDie(RunYamlScanCommand(args[0], ctx, cancel, yamlScanFlags, detectPassThroughFlagsMap))
		},
	}

	command.Flags().StringVar(&yamlScanFlags.DetectOfflineMode, DetectOfflineModeFlag, "false", "Enabled Offline Scanning")
	command.Flags().StringVar(&yamlScanFlags.BlackDuckURL, BlackDuckURLFlag, "", "Black Duck Server URL")
	command.Flags().StringVar(&yamlScanFlags.BlackDuckToken, BlackDuckTokenFlag, "", "Black Duck API Token")
	command.Flags().StringVar(&yamlScanFlags.DetectProjectName, DetectProjectNameFlag, "", "An override for the name to use for the Black Duck project. If not supplied, a project will be created with namespace name and image name and tag will be passed as version.")

	return command
}

func RunYamlScanCommand(yamlfile string, ctx context.Context, cancellationFunc context.CancelFunc, yamlScanFlags *yamlScanFlags, detectPassThroughFlagsMap map[string]interface{}) error {
	var err error
	var imageList []string

	// replace with yaml parsing
	imageList, err = yaml.getImageFromYaml(filename)
	if err != nil {
		return err
	}

	var projectName string
	var userSuppliedProjectName = yamlScanFlags.DetectProjectName
	if 0 == len(userSuppliedProjectName) {
		projectName = yamlfile
	} else {
		projectName = userSuppliedProjectName
	}

	return RunAndPrintMultipleImageScansConcurrently(ctx, cancellationFunc, imageList, detectPassThroughFlagsMap, projectName)
}
