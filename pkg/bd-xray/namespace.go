package bd_xray

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/blackducksoftware/kubectl-bd-xray/pkg/kube"
	"github.com/blackducksoftware/kubectl-bd-xray/pkg/util"
)

type NamespaceScanFlags struct {
	DetectOfflineMode string
	BlackDuckURL      string
	BlackDuckToken    string
	DetectProjectName string
	// TODO: add how many scans to process simultaneously
	// ConcurrencyLevel  string
}

func SetupNamespaceScanCommand() *cobra.Command {
	namespaceScanFlags := &NamespaceScanFlags{}

	detectPassThroughFlagsMap := map[string]interface{}{
		DetectOfflineModeFlag: &namespaceScanFlags.DetectOfflineMode,
		BlackDuckURLFlag:      &namespaceScanFlags.BlackDuckURL,
		BlackDuckTokenFlag:    &namespaceScanFlags.BlackDuckToken,
	}

	ctx, cancel := context.WithCancel(context.Background())

	command := &cobra.Command{
		Use:   "namespace NAMESPACE_NAME...",
		Short: "scan all images in a namespace",
		Long:  "scan all images in a namespace",
		Args: func(cmd *cobra.Command, args []string) error {
			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			util.DoOrDie(RunNamespaceScanCommand(args[0], ctx, cancel, namespaceScanFlags, detectPassThroughFlagsMap))
		},
	}

	command.Flags().StringVar(&namespaceScanFlags.DetectOfflineMode, DetectOfflineModeFlag, "false", "Enabled Offline Scanning")
	command.Flags().StringVar(&namespaceScanFlags.BlackDuckURL, BlackDuckURLFlag, "", "Black Duck Server URL")
	command.Flags().StringVar(&namespaceScanFlags.BlackDuckToken, BlackDuckTokenFlag, "", "Black Duck API Token")
	command.Flags().StringVar(&namespaceScanFlags.DetectProjectName, DetectProjectNameFlag, "", "An override for the name to use for the Black Duck project. If not supplied, a project will be created with namespace name and image name and tag will be passed as version.")

	return command
}

func RunNamespaceScanCommand(namespace string, ctx context.Context, cancellationFunc context.CancelFunc, namespaceScanFlags *NamespaceScanFlags, detectPassThroughFlagsMap map[string]interface{}) error {
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
	var userSuppliedProjectName = namespaceScanFlags.DetectProjectName
	if 0 == len(userSuppliedProjectName) {
		projectName = namespace
	} else {
		projectName = userSuppliedProjectName
	}

	return RunAndPrintMultipleImageScansConcurrently(ctx, cancellationFunc, imageList, detectPassThroughFlagsMap, projectName)
}
