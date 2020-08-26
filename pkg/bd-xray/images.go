package bd_xray

import (
	"fmt"

	detectApi "github.com/blackducksoftware/kubectl-bd-xray/pkg/bd-xray/detect/api"
	"github.com/spf13/cobra"
)

const (
	DetectOfflineModeFlag = "blackduck.offline.mode"
	BlackDuckURLFlag      = "blackduck.url"
	BlackDuckTokenFlag    = "blackduck.api.token"
	DetectProjectNameFlag = "detect.project.name"
	DetectVersionNameFlag = "detect.project.version.name"
)

type ImageScanFlags struct {
	DetectOfflineMode string
	BlackDuckURL      string
	BlackDuckToken    string
	DetectProjectName string
	DetectVersionName string
	LoggingLevel      string
}

func SetupImageScanCommand() *cobra.Command {
	imageScanFlags := &ImageScanFlags{}

	flagMap := map[string]interface{}{
		DetectOfflineModeFlag: imageScanFlags.DetectOfflineMode,
		BlackDuckURLFlag:      &imageScanFlags.BlackDuckURL,
		BlackDuckTokenFlag:    &imageScanFlags.BlackDuckToken,
		DetectProjectNameFlag: &imageScanFlags.DetectProjectName,
		DetectVersionNameFlag: &imageScanFlags.DetectVersionName,
	}

	command := &cobra.Command{
		Use:   "image DOCKER_IMAGE",
		Short: "",
		Long:  "",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			// for _, image := range args {
			// 	fmt.Printf("Detect!")
			// 	go RunImageScanCommand(args, flagMap)
			// }
			RunImageScanCommand(args, flagMap)
		},
	}

	command.Flags().StringVar(&imageScanFlags.DetectOfflineMode, DetectOfflineModeFlag, "false", "scan the image offline")
	command.Flags().StringVar(&imageScanFlags.BlackDuckURL, BlackDuckURLFlag, "", "Blackduck Server URL")
	command.Flags().StringVar(&imageScanFlags.BlackDuckToken, BlackDuckTokenFlag, "", "BlackDuck API Token")

	return command
}

func RunImageScanCommand(args []string, flagMap map[string]interface{}) error {

	detectClient := detectApi.NewDefaultClient()
	detectClient.DownloadDetectIfNotExists()

	flagsToPassToDetect := ""
	for flagName, flagVal := range flagMap {
		castFlagVal := *flagVal.(*string)
		if castFlagVal == "" {
			continue
		}
		flagsToPassToDetect += fmt.Sprintf("--%s=%v ", flagName, castFlagVal)
	}

	err := detectClient.RunImageScan(args[0], flagsToPassToDetect)
	return err
}
