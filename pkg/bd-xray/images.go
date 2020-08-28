package bd_xray

import (
	"context"
	"fmt"
	"time"

	"github.com/jedib0t/go-pretty/table"
	"github.com/oklog/run"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/blackducksoftware/kubectl-bd-xray/pkg/detect"
	"github.com/blackducksoftware/kubectl-bd-xray/pkg/util"
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
	// TODO: add how many scans to process simultaneously
	// ConcurrencyLevel  string
}

func SetupImageScanCommand() *cobra.Command {
	imageScanFlags := &ImageScanFlags{}

	detectPassThroughFlagsMap := map[string]interface{}{
		DetectOfflineModeFlag: &imageScanFlags.DetectOfflineMode,
		BlackDuckURLFlag:      &imageScanFlags.BlackDuckURL,
		BlackDuckTokenFlag:    &imageScanFlags.BlackDuckToken,
	}

	ctx, cancel := context.WithCancel(context.Background())

	command := &cobra.Command{
		Use:   "images DOCKER_IMAGE...",
		Short: "scan multiple images",
		Long:  "scan multiple images",
		Args: func(cmd *cobra.Command, args []string) error {
			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			util.DoOrDie(RunAndPrintMultipleImageScansConcurrently(ctx, cancel, args, detectPassThroughFlagsMap, imageScanFlags.DetectProjectName))
		},
	}

	command.Flags().StringVar(&imageScanFlags.DetectOfflineMode, DetectOfflineModeFlag, "false", "Enabled Offline Scanning")
	command.Flags().StringVar(&imageScanFlags.BlackDuckURL, BlackDuckURLFlag, "", "Black Duck Server URL")
	command.Flags().StringVar(&imageScanFlags.BlackDuckToken, BlackDuckTokenFlag, "", "Black Duck API Token")
	command.Flags().StringVar(&imageScanFlags.DetectProjectName, DetectProjectNameFlag, "", "An override for the name to use for the Black Duck project. If not supplied, a project will be created for each image")

	return command
}

func RunAndPrintMultipleImageScansConcurrently(ctx context.Context, cancellationFunc context.CancelFunc, imageList []string, detectPassThroughFlagsMap map[string]interface{}, projectName string) error {
	var err error

	detectClient := detect.NewDefaultClient()
	err = detectClient.DownloadDetectIfNotExists()
	if err != nil {
		return err
	}
	err = detectClient.SetupPersistentDockerInspectorServices()
	if err != nil {
		return err
	}

	scanStatusTableValues := make(chan *ScanStatusTableValues)
	doneChan := make(chan bool, 1)

	err = RunPrinterConcurrently(cancellationFunc, scanStatusTableValues, doneChan)
	if err != nil {
		return err
	}

	err = RunMultipleImageScansConcurrently(ctx, cancellationFunc, detectClient, imageList, detectPassThroughFlagsMap, scanStatusTableValues, projectName)
	if err != nil {
		return err
	}

	BlockOnDoneChan(doneChan)
	return nil
}

func RunPrinterConcurrently(cancellationFunc context.CancelFunc, scanStatusTableValues <-chan *ScanStatusTableValues, doneChan chan<- bool) error {
	var printerGoRoutine run.Group
	printerGoRoutine.Add(func() error {
		PrintScanStatusTable(scanStatusTableValues, doneChan)
		return nil
	}, func(error) {
		cancellationFunc()
	})
	log.Tracef("starting printer goroutine")
	go printerGoRoutine.Run()
	return nil
}

func BlockOnDoneChan(doneChan chan bool) {
	log.Tracef("blocking on done channel")
	select {
	case <-doneChan:
		log.Infof("All done!")
	}
}

func RunMultipleImageScansConcurrently(ctx context.Context, cancellationFunc context.CancelFunc, detectClient *detect.Client, imageList []string, detectPassThroughFlagsMap map[string]interface{}, scanStatusTableValues chan *ScanStatusTableValues, projectName string) error {
	var err error

	var goRoutineGroup run.Group

	for _, image := range imageList {
		image := image
		log.Infof("Scanning image: %s", image)
		goRoutineGroup.Add(func() error {
			return RunImageScanCommand(ctx, detectClient, image, detectPassThroughFlagsMap, scanStatusTableValues, projectName)
		}, func(error) {
			cancellationFunc()
		})
	}

	log.Tracef("starting scanning goroutines")
	err = goRoutineGroup.Run()
	if err != nil {
		log.Fatalf("FATAL ERROR: %+v", err)
	}

	log.Tracef("closing the output channel")
	close(scanStatusTableValues)
	return err
}

// RunImageScanCommand
// https://synopsys.atlassian.net/wiki/spaces/INTDOCS/pages/631374044/Detect+Properties
func RunImageScanCommand(ctx context.Context, detectClient *detect.Client, fullImageName string, detectPassThroughFlagsMap map[string]interface{}, scanStatusTableValues chan *ScanStatusTableValues, projectName string) error {

	var err error

	detectPassThroughFlags := ""
	for flagName, flagVal := range detectPassThroughFlagsMap {
		castFlagVal := *flagVal.(*string)
		if castFlagVal == "" {
			continue
		}
		detectPassThroughFlags += fmt.Sprintf("--%s=%v ", flagName, castFlagVal)
	}

	imageName := util.ParseImageName(fullImageName)
	imageTag := util.ParseImageTag(fullImageName)
	// // needed in order to calculate the sha
	// err = detectClient.DockerCLIClient.PullDockerImage(fullImageName)
	// if err != nil {
	// 	return err
	// }
	// imageSha, err := detectClient.DockerCLIClient.GetImageSha(fullImageName)
	// if err != nil {
	// 	return err
	// }
	// a unique string, but something that's human readable, i.e.: TIMESTAMP_NAME_TAG_RANDOMSTRING
	timestampUniqueSanitizedString := util.SanitizeString(fmt.Sprintf("%s_%s_%s_%s", time.Now().Format("20060102150405"), imageName, imageTag, util.GenerateRandomString(16)))
	uniqueOutputDirName := fmt.Sprintf("%s/%s", detect.DefaultDetectBlackduckDirectory, timestampUniqueSanitizedString)
	log.Tracef("output dir is: %s", uniqueOutputDirName)

	err = detectClient.RunImageScan(fullImageName, projectName, imageName, imageTag, uniqueOutputDirName, detectPassThroughFlags)
	if err != nil {
		return err
	}

	// parsing output infos
	log.Infof("finding scan status file from uniqueOutputDirName: %s", uniqueOutputDirName)
	statusFilePath, err := detect.FindScanStatusFile(uniqueOutputDirName)
	if err != nil {
		return err
	}
	log.Infof("statusFilePath is known to be: %s", statusFilePath)
	statusJSON, err := detect.ParseStatusJSONFile(statusFilePath)
	if err != nil {
		return err
	}
	locations := detect.FindLocationFromStatus(statusJSON)
	if len(locations) == 0 {
		// TODO: how to handle this better??
		log.Warnf("no location found; either running offline mode or something went wrong")
		return nil
	}
	location := locations[0]
	log.Infof("location in Black Duck: %s", location)

	// TODO: add a column in table for where detect logs so users can examine afterwards if needed
	outputRow := &ScanStatusTableValues{ImageName: fullImageName, BlackDuckURL: location}
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

type ScanStatusTableValues struct {
	ImageName    string
	BlackDuckURL string
}

func PrintScanStatusTable(tableValues <-chan *ScanStatusTableValues, printingFinishedChannel chan<- bool) {
	log.Tracef("inside table printer")
	t := table.NewWriter()
	// t.SetOutputMirror(os.Stdout)
	// t.SetAutoIndex(true)
	t.AppendHeader(table.Row{"Image Name", "BlackDuck URL"})

	// process output structs concurrently
	log.Tracef("waiting for values over channel")
	for tableValue := range tableValues {
		log.Tracef("processing table value for image: %s, url: %s", tableValue.ImageName, tableValue.BlackDuckURL)
		t.AppendRow([]interface{}{
			fmt.Sprintf("%s", tableValue.ImageName),
			fmt.Sprintf("%s", tableValue.BlackDuckURL),
		})
		fmt.Printf("Intermediate Table: \n%s\n\n", t.Render())
	}
	// TODO: to be able to render concurrently
	log.Tracef("rendering the table")
	fmt.Printf("\n%s\n\n", t.Render())
	printingFinishedChannel <- true
	close(printingFinishedChannel)
}
