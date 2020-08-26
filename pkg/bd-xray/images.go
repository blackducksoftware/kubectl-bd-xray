package bd_xray

import (
	"context"
	"fmt"

	"github.com/blackducksoftware/kubectl-bd-xray/pkg/detect"
	"github.com/oklog/run"
	log "github.com/sirupsen/logrus"
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
	// TODO: add how many scans to process simultaneously
	// ConcurrencyLevel  string
}

func SetupImageScanCommand() *cobra.Command {
	imageScanFlags := &ImageScanFlags{}

	flagMap := map[string]interface{}{
		DetectOfflineModeFlag: &imageScanFlags.DetectOfflineMode,
		BlackDuckURLFlag:      &imageScanFlags.BlackDuckURL,
		BlackDuckTokenFlag:    &imageScanFlags.BlackDuckToken,
		DetectProjectNameFlag: &imageScanFlags.DetectProjectName,
		DetectVersionNameFlag: &imageScanFlags.DetectVersionName,
	}

	ctx, cancel := context.WithCancel(context.Background())
	var goRoutineGroup run.Group

	command := &cobra.Command{
		Use:   "image DOCKER_IMAGE...",
		Short: "",
		Long:  "",
		Args: func(cmd *cobra.Command, args []string) error {
			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			for _, image := range args {
				image := image
				log.Infof("Scanning image: %s", image)
				goRoutineGroup.Add(func() error {
					return RunImageScanCommand(image, flagMap, ctx)
				}, func(error) {
					cancel()
				})
			}

			err := goRoutineGroup.Run()
			if err != nil {
				log.Fatalf("FATAL ERROR: %+v", err)
			}
		},
	}

	command.Flags().StringVar(&imageScanFlags.DetectOfflineMode, DetectOfflineModeFlag, "false", "Enabled Offline Scanning")
	command.Flags().StringVar(&imageScanFlags.BlackDuckURL, BlackDuckURLFlag, "", "Blackduck Server URL")
	command.Flags().StringVar(&imageScanFlags.BlackDuckToken, BlackDuckTokenFlag, "", "BlackDuck API Token")
	command.Flags().StringVar(&imageScanFlags.DetectProjectName, DetectProjectNameFlag, "", "An override for the name to use for the Black Duck project. f not supplied, Detect will attempt to use the tools to figure out a reasonable project name.")
	command.Flags().StringVar(&imageScanFlags.DetectVersionName, DetectVersionNameFlag, "", "An override for the version to use for the Black Duck project. If not supplied, Detect will attempt to use the tools to figure out a reasonable version name. If that fails, the current date will be used.")

	return command
}

func RunImageScanCommand(image string, flagMap map[string]interface{}, ctx context.Context) error {
	detectClient := detect.NewDefaultClient()
	detectClient.DownloadDetectIfNotExists()

	flagsToPassToDetect := ""
	for flagName, flagVal := range flagMap {
		castFlagVal := *flagVal.(*string)
		if castFlagVal == "" {
			continue
		}
		flagsToPassToDetect += fmt.Sprintf("--%s=%v ", flagName, castFlagVal)
	}

	err := detectClient.RunImageScan(image, flagsToPassToDetect)
	return err
}

func ParseForBomURL() {

}

// docker run -v
