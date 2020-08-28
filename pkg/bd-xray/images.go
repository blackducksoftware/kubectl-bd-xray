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
	"github.com/blackducksoftware/kubectl-bd-xray/pkg/registries"
	"github.com/blackducksoftware/kubectl-bd-xray/pkg/remediation"
	"github.com/blackducksoftware/kubectl-bd-xray/pkg/util"
)

const (
	DetectOfflineModeFlagName                    = "blackduck.offline.mode"
	BlackDuckURLFlagName                         = "blackduck.url"
	BlackDuckTokenFlagName                       = "blackduck.api.token"
	DetectProjectNameFlagName                    = "detect.project.name"
	DetectVersionNameFlagName                    = "detect.project.version.name"
	CleanupPersistentDockerInspectorServicesName = "cleanup"
)

type CommonFlags struct {
	DetectOfflineMode                        string
	BlackDuckURL                             string
	BlackDuckToken                           string
	DetectProjectName                        string // TODO: this is handle specially, not just a passthrough
	CleanupPersistentDockerInspectorServices bool
	// TODO: add how many scans to process simultaneously
	// ConcurrencyLevel  string
}

func SetupImageScanCommand() *cobra.Command {
	commonFlags := &CommonFlags{}

	detectPassThroughFlagsMap := map[string]interface{}{
		DetectOfflineModeFlagName: &commonFlags.DetectOfflineMode,
		BlackDuckURLFlagName:      &commonFlags.BlackDuckURL,
		BlackDuckTokenFlagName:    &commonFlags.BlackDuckToken,
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
			util.DoOrDie(RunAndPrintMultipleImageScansConcurrently(ctx, cancel, args, detectPassThroughFlagsMap, commonFlags.DetectProjectName, commonFlags.CleanupPersistentDockerInspectorServices))
		},
	}

	command.Flags().StringVar(&commonFlags.DetectOfflineMode, DetectOfflineModeFlagName, "false", "Enabled Offline Scanning")
	command.Flags().StringVar(&commonFlags.BlackDuckURL, BlackDuckURLFlagName, "", "Black Duck Server URL")
	command.Flags().StringVar(&commonFlags.BlackDuckToken, BlackDuckTokenFlagName, "", "Black Duck API Token")
	command.Flags().StringVar(&commonFlags.DetectProjectName, DetectProjectNameFlagName, "", "An override for the name to use for the Black Duck project. If not supplied, a project will be created for each image")
	command.Flags().BoolVarP(&commonFlags.CleanupPersistentDockerInspectorServices, CleanupPersistentDockerInspectorServicesName, "c", true, "Clean up the docker inspector services")

	return command
}

func RunAndPrintMultipleImageScansConcurrently(ctx context.Context, cancellationFunc context.CancelFunc, imageList []string, detectPassThroughFlagsMap map[string]interface{}, projectName string, cleanup bool) error {
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
	if cleanup {
		defer detectClient.StopAndCleanupPersistentDockerInspectorServices()
	}

	scanStatusRowChan := make(chan *ScanStatusRow)
	doneChan := make(chan bool, 1)

	err = RunPrinterConcurrently(cancellationFunc, scanStatusRowChan, doneChan)
	if err != nil {
		return err
	}

	err = RunMultipleImageScansConcurrently(ctx, cancellationFunc, detectClient, imageList, detectPassThroughFlagsMap, scanStatusRowChan, projectName)
	if err != nil {
		return err
	}

	BlockOnDoneChan(doneChan)

	return nil
}

func RunPrinterConcurrently(cancellationFunc context.CancelFunc, scanStatusTableValues <-chan *ScanStatusRow, doneChan chan<- bool) error {
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

func RunMultipleImageScansConcurrently(ctx context.Context, cancellationFunc context.CancelFunc, detectClient *detect.Client, imageList []string, detectPassThroughFlagsMap map[string]interface{}, scanStatusRowChan chan *ScanStatusRow, projectName string) error {
	var err error

	var goRoutineGroup run.Group

	for _, image := range imageList {
		image := image
		log.Infof("Scanning image: %s", image)
		scanStatusRow := &ScanStatusRow{}
		goRoutineGroup.Add(func() error {
			return RunImageScanCommand(ctx, detectClient, image, detectPassThroughFlagsMap, scanStatusRow, scanStatusRowChan, projectName)
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
	close(scanStatusRowChan)
	return err
}

// RunImageScanCommand
// https://synopsys.atlassian.net/wiki/spaces/INTDOCS/pages/631374044/Detect+Properties
func RunImageScanCommand(ctx context.Context, detectClient *detect.Client, fullImageName string, detectPassThroughFlagsMap map[string]interface{}, scanStatusRow *ScanStatusRow, scanStatusRowChan chan *ScanStatusRow, projectName string) error {

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

	// fill in all the rows
	scanStatusRow.ImageName = imageName
	scanStatusRow.ImageTag = imageTag
	scanStatusRow.BlackDuckURL = location
	// TODO: add a column in table for where detect logs so users can examine afterwards if needed

	var images []remediation.Image
	oneImage := remediation.Image{FullPath: fullImageName, URL: "docker.io", Name: imageName, Version: imageTag}
	images = append(images, oneImage)

	newRegistries := registries.ImageRegistries{}
	newRegistries.DefaultRegistries()
	latestInfo := remediation.GetLatestVersionsForImages(images, newRegistries)

	var latestVersion string
	for _, inf := range latestInfo {
		latestVersion = inf.LatestVersion
	}
	scanStatusRow.LatestAvailableImageVersion = latestVersion

	log.Infof("Sending output to Table Printer %s %s %s", scanStatusRow.ImageName, scanStatusRow.BlackDuckURL, scanStatusRow.LatestAvailableImageVersion)
	scanStatusRowChan <- scanStatusRow

	return err
}

type ScanStatusRow struct {
	ImageName                   string
	ImageTag                    string
	ImageSha                    string
	BlackDuckURL                string
	LatestAvailableImageVersion string
}

func PrintScanStatusTable(scanStatusRowChan <-chan *ScanStatusRow, printingFinishedChannel chan<- bool) {
	log.Tracef("inside table printer")
	t := table.NewWriter()
	// t.SetOutputMirror(os.Stdout)
	// t.SetAutoIndex(true)
	t.AppendHeader(table.Row{"Image Name", "Image Tag", "BlackDuck URL", "Latest Available Image Tag"})

	// process output structs concurrently
	log.Tracef("waiting for values over channel")
	for row := range scanStatusRowChan {
		log.Tracef("processing table value for image: %s, url: %s", row.ImageName, row.BlackDuckURL)
		t.AppendRow([]interface{}{
			fmt.Sprintf("%s", row.ImageName),
			fmt.Sprintf("%s", row.ImageTag),
			fmt.Sprintf("%s", row.BlackDuckURL),
			fmt.Sprintf("%s", row.LatestAvailableImageVersion),
		})
		fmt.Printf("Intermediate Table: \n%s\n\n", t.Render())
	}
	// TODO: to be able to render concurrently
	log.Tracef("rendering the table")
	fmt.Printf("\n%s\n\n", t.Render())
	printingFinishedChannel <- true
	close(printingFinishedChannel)
}
