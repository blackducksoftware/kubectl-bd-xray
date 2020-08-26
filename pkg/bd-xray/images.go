package bd_xray

import (
	"context"
	"fmt"
	"os"

	"github.com/blackducksoftware/kubectl-bd-xray/pkg/detect"
	"github.com/blackducksoftware/kubectl-bd-xray/pkg/util"
	"github.com/oklog/run"
	"github.com/pkg/errors"
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
	// var mainGroup run.Group
	var goRoutineGroup run.Group
	var printerGoRoutine run.Group
	outputChan := make(chan *util.ScanStatusTableValues)
	printingFinishedChannel := make(chan bool, 1)

	command := &cobra.Command{
		Use:   "image DOCKER_IMAGE...",
		Short: "",
		Long:  "",
		Args: func(cmd *cobra.Command, args []string) error {
			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {

			printerGoRoutine.Add(func() error {
				util.PrintScanStatusTable(outputChan, printingFinishedChannel)
				return nil
			}, func(error) {
				cancel()
			})

			for _, image := range args {
				image := image
				log.Infof("Scanning image: %s", image)
				goRoutineGroup.Add(func() error {
					return RunImageScanCommand(ctx, image, flagMap, outputChan)
				}, func(error) {
					cancel()
				})
			}

			log.Debugf("starting printer goroutine")
			var err error

			go printerGoRoutine.Run()

			log.Debugf("starting scanning goroutines")

			err = goRoutineGroup.Run()
			if err != nil {
				log.Fatalf("FATAL ERROR: %+v", err)
			}
			log.Debugf("closing the output channel")

			close(outputChan)
			// time.Sleep(10 * time.Second)

			// TODO: Block on printerGoRoutine
			select {
			case msg1 := <-printingFinishedChannel:
				fmt.Printf("Done: %+v", msg1)
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

func RunImageScanCommand(ctx context.Context, image string, flagMap map[string]interface{}, scanStatusTableValues chan *util.ScanStatusTableValues) error {
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

	homeDir, _ := os.UserHomeDir()
	outputDirName := fmt.Sprintf("%s/blackduck/%s", homeDir, util.GenerateRandomString(16))
	log.Infof("output dir is: %s", outputDirName)
	// actually scan
	log.Tracef("starting image scan")
	err := detectClient.RunImageScan(image, outputDirName, flagsToPassToDetect)
	if err != nil {
		return err
	}

	// parsing output infos
	log.Debugf("finding scan status file")
	statusFilePath, err := detect.FindScanStatusFile(outputDirName)
	if err != nil {
		return err
	}
	log.Infof("statusFilePath is known to be: %s", statusFilePath)
	statusJSON, err := detect.ParseStatusJSONFile(statusFilePath)
	if err != nil {
		return err
	}
	// log.Infof("")
	locations := detect.FindLocationFromStatus(statusJSON)
	if len(locations) == 0 {
		return errors.Errorf("no location found")
	}
	location := locations[0]
	log.Infof("location in Black Duck: %s", location)

	outputRow := &util.ScanStatusTableValues{ImageName: image, BlackDuckURL: location}
	log.Infof("Sending output to Table Printer %s %s", outputRow.ImageName, outputRow.BlackDuckURL)
	// scanStatusTableValues <- outputRow

	select {
	case <-ctx.Done():
		return ctx.Err()
	case scanStatusTableValues <- outputRow:
		log.Debug("Got output")
	}

	return err
}
