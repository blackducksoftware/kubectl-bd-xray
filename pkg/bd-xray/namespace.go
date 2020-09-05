package bd_xray

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/blackducksoftware/kubectl-bd-xray/pkg/kube"
	"github.com/blackducksoftware/kubectl-bd-xray/pkg/utils"
)

func SetupNamespaceScanCommand() *cobra.Command {
	commonFlags := &CommonFlags{}

	detectPassThroughFlagsMap := map[string]interface{}{
		DetectOfflineModeFlagName: &commonFlags.DetectOfflineMode,
		BlackDuckURLFlagName:      &commonFlags.BlackDuckURL,
		BlackDuckTokenFlagName:    &commonFlags.BlackDuckToken,
	}

	ctx, cancel := context.WithCancel(context.Background())

	command := &cobra.Command{
		Use:   "namespace NAMESPACE_NAME",
		Short: "scan all images in a namespace",
		Long:  "scan all images in a namespace",
		Args: func(cmd *cobra.Command, args []string) error {
			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			utils.DoOrDie(RunNamespaceScanCommand(args[0], ctx, cancel, commonFlags, detectPassThroughFlagsMap))
		},
	}

	command.Flags().StringVar(&commonFlags.DetectOfflineMode, DetectOfflineModeFlagName, "false", "Enabled Offline Scanning")
	command.Flags().StringVar(&commonFlags.BlackDuckURL, BlackDuckURLFlagName, "", "Black Duck Server URL")
	command.Flags().StringVar(&commonFlags.BlackDuckToken, BlackDuckTokenFlagName, "", "Black Duck API Token")
	command.Flags().StringVar(&commonFlags.DetectProjectName, DetectProjectNameFlagName, "", "An override for the name to use for the Black Duck project. If not supplied, a project will be created with namespace name and image name and tag will be passed as version.")
	command.Flags().BoolVarP(&commonFlags.CleanupPersistentDockerInspectorServices, CleanupPersistentDockerInspectorServicesName, "c", true, "Clean up the docker inspector services")

	return command
}

func RunNamespaceScanCommand(namespace string, ctx context.Context, cancellationFunc context.CancelFunc, commonFlags *CommonFlags, detectPassThroughFlagsMap map[string]interface{}) error {
	var err error
	var imageList []string

	cli, err := kube.NewDefaultClient()
	if err != nil {
		return err
	}
	imageList, err = cli.GetImagesFromNamespace(context.Background(), namespace)
	if err != nil {
		return err
	}

	var projectName string
	var userSuppliedProjectName = commonFlags.DetectProjectName
	if 0 == len(userSuppliedProjectName) {
		projectName = namespace
	} else {
		projectName = userSuppliedProjectName
	}

	return RunAndPrintMultipleImageScansConcurrently(ctx, cancellationFunc, imageList, detectPassThroughFlagsMap, projectName, commonFlags.CleanupPersistentDockerInspectorServices)
}
