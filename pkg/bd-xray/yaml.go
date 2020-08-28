package bd_xray

import (
	"context"

	"path/filepath"
	"github.com/spf13/cobra"

	"github.com/blackducksoftware/kubectl-bd-xray/pkg/util"
	"github.com/blackducksoftware/kubectl-bd-xray/pkg/yaml"
)

func SetupYamlScanCommand() *cobra.Command {
	commonFlags := &CommonFlags{}

	detectPassThroughFlagsMap := map[string]interface{}{
		DetectOfflineModeFlagName: &commonFlags.DetectOfflineMode,
		BlackDuckURLFlagName:      &commonFlags.BlackDuckURL,
		BlackDuckTokenFlagName:    &commonFlags.BlackDuckToken,
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
			util.DoOrDie(RunYamlScanCommand(args[0], ctx, cancel, commonFlags, detectPassThroughFlagsMap))
		},
	}

	command.Flags().StringVar(&commonFlags.DetectOfflineMode, DetectOfflineModeFlagName, "false", "Enabled Offline Scanning")
	command.Flags().StringVar(&commonFlags.BlackDuckURL, BlackDuckURLFlagName, "", "Black Duck Server URL")
	command.Flags().StringVar(&commonFlags.BlackDuckToken, BlackDuckTokenFlagName, "", "Black Duck API Token")
	command.Flags().StringVar(&commonFlags.DetectProjectName, DetectProjectNameFlagName, "", "An override for the name to use for the Black Duck project. If not supplied, a project will be created with yaml name and image name and tag will be passed as version.")

	return command
}

func RunYamlScanCommand(yamlfile string, ctx context.Context, cancellationFunc context.CancelFunc, commonFlags *CommonFlags, detectPassThroughFlagsMap map[string]interface{}) error {
	var err error
	var imageList []string

	// replace with yaml parsing
	imageList, err = yaml.GetImageFromYaml(yamlfile)
	if err != nil {
		return err
	}

	var projectName string
	var userSuppliedProjectName = commonFlags.DetectProjectName
	if 0 == len(userSuppliedProjectName) {
		projectName = util.SanitizeString(filepath.Base(yamlfile))
	} else {
		projectName = userSuppliedProjectName
	}

	return RunAndPrintMultipleImageScansConcurrently(ctx, cancellationFunc, imageList, detectPassThroughFlagsMap, projectName, commonFlags.CleanupPersistentDockerInspectorServices)
}
