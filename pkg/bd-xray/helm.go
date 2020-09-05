package bd_xray

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/blackducksoftware/kubectl-bd-xray/pkg/helm"
	"github.com/blackducksoftware/kubectl-bd-xray/pkg/utils"
	"github.com/blackducksoftware/kubectl-bd-xray/pkg/yaml"
)

func SetupHelmScanCommand() *cobra.Command {
	commonFlags := &CommonFlags{}

	detectPassThroughFlagsMap := map[string]interface{}{
		DetectOfflineModeFlagName: &commonFlags.DetectOfflineMode,
		BlackDuckURLFlagName:      &commonFlags.BlackDuckURL,
		BlackDuckTokenFlagName:    &commonFlags.BlackDuckToken,
	}

	ctx, cancel := context.WithCancel(context.Background())

	command := &cobra.Command{
		Use:   "helm CHART_URL",
		Short: "scan all images in a Chart",
		Long:  "scan all images in a Chart",
		Args: func(cmd *cobra.Command, args []string) error {
			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			utils.DoOrDie(RunHelmScanCommand(args, ctx, cancel, commonFlags, detectPassThroughFlagsMap))
		},
	}

	command.Flags().StringVar(&commonFlags.DetectOfflineMode, DetectOfflineModeFlagName, "false", "Enabled Offline Scanning")
	command.Flags().StringVar(&commonFlags.BlackDuckURL, BlackDuckURLFlagName, "", "Black Duck Server URL")
	command.Flags().StringVar(&commonFlags.BlackDuckToken, BlackDuckTokenFlagName, "", "Black Duck API Token")
	command.Flags().StringVar(&commonFlags.DetectProjectName, DetectProjectNameFlagName, "", "An override for the name to use for the Black Duck project. If not supplied, a project will be created with chart name and image name and tag will be passed as version.")
	command.Flags().BoolVarP(&commonFlags.CleanupPersistentDockerInspectorServices, CleanupPersistentDockerInspectorServicesName, "c", true, "Clean up the docker inspector services")

	return command
}

func RunHelmScanCommand(charts []string, ctx context.Context, cancellationFunc context.CancelFunc, commonFlags *CommonFlags, detectPassThroughFlagsMap map[string]interface{}) error {
	var imageList []string

	for _, chart := range charts {
		chartOutput, err := helm.TemplateChart(chart)
		if err != nil {
			return err
		}
		chartImages := yaml.GetImageFromYamlString(chartOutput)

		imageList = append(imageList, chartImages...)
	}

	return RunAndPrintMultipleImageScansConcurrently(ctx, cancellationFunc, imageList, detectPassThroughFlagsMap, commonFlags.DetectProjectName, commonFlags.CleanupPersistentDockerInspectorServices)
}
