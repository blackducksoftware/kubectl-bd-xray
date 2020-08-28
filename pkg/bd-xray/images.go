package bd_xray

import (
	"context"
	"fmt"
	"time"

	"github.com/jedib0t/go-pretty/v6/table"
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

	rowValuesGetter := &RowValuesGetter{make(chan *BlackDuckURLRowGetter, len(imageList))}
	doneChan := make(chan bool, 1)

	err = RunPrinterConcurrently(cancellationFunc, rowValuesGetter, imageList, doneChan)
	if err != nil {
		return err
	}

	err = RunMultipleImageScansConcurrently(ctx, cancellationFunc, detectClient, imageList, detectPassThroughFlagsMap, rowValuesGetter, projectName)
	if err != nil {
		return err
	}

	BlockOnDoneChan(doneChan)
	return nil
}

func RunPrinterConcurrently(cancellationFunc context.CancelFunc, rowValuesGetter *RowValuesGetter, imageNames []string, doneChan chan<- bool) error {
	var printerGoRoutine run.Group
	printerGoRoutine.Add(func() error {
		PrintScanStatusTableChan(rowValuesGetter, imageNames, doneChan)
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

func RunMultipleImageScansConcurrently(ctx context.Context, cancellationFunc context.CancelFunc, detectClient *detect.Client, imageList []string, detectPassThroughFlagsMap map[string]interface{}, rowValuesGetter *RowValuesGetter, projectName string) error {
	var err error

	var goRoutineGroup run.Group

	for _, image := range imageList {
		image := image
		log.Infof("Scanning image: %s", image)
		goRoutineGroup.Add(func() error {
			return RunImageScanCommand(ctx, detectClient, image, detectPassThroughFlagsMap, rowValuesGetter, projectName)
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
	close(rowValuesGetter.BlackDuckURLs)
	return err
}

// RunImageScanCommand
// https://synopsys.atlassian.net/wiki/spaces/INTDOCS/pages/631374044/Detect+Properties
func RunImageScanCommand(ctx context.Context, detectClient *detect.Client, fullImageName string, detectPassThroughFlagsMap map[string]interface{}, rowValuesGetter *RowValuesGetter, projectName string) error {

	// TODO: add a column in table for where detect logs so users can examine afterwards if needed
	outputRow := &BlackDuckURLRowGetter{RowID: fullImageName, BlackDuckURL: "dummy_url"}
	log.Infof("Sending output to Table Printer %s %s", outputRow.RowID, outputRow.BlackDuckURL)
	// scanStatusTableValues <- outputRow

	rowValuesGetter.BlackDuckURLs <- outputRow
	log.Tracef("outputRow is set to : %s, %s", outputRow.RowID, outputRow.BlackDuckURL)
	log.Tracef("rowValuesGetter is set to : %s", rowValuesGetter.BlackDuckURLs)

	return nil
}

// RunImageScanCommand TODO - Change this back to the original
// https://synopsys.atlassian.net/wiki/spaces/INTDOCS/pages/631374044/Detect+Properties
func RunImageScanCommandOld(ctx context.Context, detectClient *detect.Client, fullImageName string, detectPassThroughFlagsMap map[string]interface{}, rowValuesGetter *RowValuesGetter, projectName string) error {

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
	outputRow := &BlackDuckURLRowGetter{RowID: fullImageName, BlackDuckURL: location}
	log.Infof("Sending output to Table Printer %s %s", outputRow.RowID, outputRow.BlackDuckURL)
	// scanStatusTableValues <- outputRow

	rowValuesGetter.BlackDuckURLs <- outputRow
	log.Tracef("outputRow is set to : %s, %s", outputRow.RowID, outputRow.BlackDuckURL)
	log.Tracef("rowValuesGetter is set to : %+v", rowValuesGetter.BlackDuckURLs)

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

type RowValuesGetter struct {
	BlackDuckURLs chan *BlackDuckURLRowGetter
	// UpgradedImages chan UpgradedImageRowGetter
}

type BlackDuckURLRowGetter struct {
	RowID        string
	BlackDuckURL string
}

type UpgradedImageRowGetter struct {
	RowID string
	// UpgradedImage string
}

type RowData struct {
	ImageName    string
	BlackDuckURL string
	// UpgradedImage string
	CloseChan chan bool
}

func PrintScanStatusTableChan(rowValuesGetter *RowValuesGetter, rowIDs []string, printingFinishedChannel chan<- bool) {
	log.Tracef("inside table printer")
	t := table.NewWriter()
	// t.SetOutputMirror(os.Stdout)
	// t.SetAutoIndex(true)
	t.AppendHeader(table.Row{"Image Name", "BlackDuck URL"})

	// process output structs concurrently
	log.Tracef("waiting for values over channel")

	// Initialize the Master Table - where rowID = imageName
	allRows := make(map[string]*RowData)
	for _, rowID := range rowIDs {
		allRows[rowID] = &RowData{
			ImageName:    rowID,
			BlackDuckURL: "",
			CloseChan:    make(chan bool, 0),
			// UpgradedImage: "",
		}
	}

	// Spin off a bunch of routines to get data and populate the Master Table

	go func(allRows map[string]*RowData) {
		log.Tracef("before the for loop")
		// BlackDuckURL objects - TODO routine
		for {
			log.Tracef("inside the for loop")
			select {
			case val := <-rowValuesGetter.BlackDuckURLs:
				log.Tracef("inside select")
				log.Tracef("waiting for blackduckurl")
				allRows[val.RowID].BlackDuckURL = val.BlackDuckURL
				// if all rows are complete, close this row chan
				// if len(allRows[val.RowID].UpgradedImage) != 0 {
				log.Tracef("close chan value before: %b", allRows[val.RowID].CloseChan)
				allRows[val.RowID].CloseChan <- true
				log.Tracef("close chan set to: %b", allRows[val.RowID].CloseChan)
				return
				// close(allRows[val.RowID].CloseChan)
				// }
			}
		}
	}(allRows)

	// go func(allRows map[string]*RowData) {
	// 	// UpgradedImage objects - TODO routine
	// 	for val := range rowValuesGetter.UpgradedImages {
	// 		allRows[val.RowID].UpgradedImage = val.UpgradedImage
	// 		// if all rows are complete, close this row chan
	// 		if len(allRows[val.RowID].BlackDuckURL) != 0 {
	// 			allRows[val.RowID].CloseChan <- true
	// 			close(allRows[val.RowID].CloseChan)
	// 		}
	// 	}
	// }(allRows)

	// var g run.Group
	for rowID, rowData := range allRows {
		rowID := rowID
		rowData := rowData
		log.Tracef("inside appending rows")
		log.Tracef("waiting for rowID: %s, rowData: %s", rowID, rowData)

		// g.Add(func() error {
		// <-rowData.CloseChan

		log.Tracef("row has been processed completely, so we can render it")
		t.AppendRow([]interface{}{
			fmt.Sprintf("%s", rowData.ImageName),
			fmt.Sprintf("%s", rowData.BlackDuckURL),
			// fmt.Sprintf("%s", rowData.UpgradedImage),
		})
		fmt.Printf("Intermediate Table: \n%s\n\n", t.Render())
		// 	return nil
		// }, func(error) {
		//
		// })
	}

	err := g.Run()
	if err != nil {
		util.DoOrDie(err)
	}

	// a way to see if the entire table is done.
	log.Tracef("rendering the table")
	fmt.Printf("\n%s\n\n", t.Render())
	printingFinishedChannel <- true
	close(printingFinishedChannel)
}

// func PrintScanStatusProgress(tableValues ColChanStruct, printingFinishedChannel chan<- bool, numOfImages int64, autoStop bool) {
// 	fmt.Printf("Tracking Progress of %d trackers ...\n\n", numOfImages)
//
// 	pw := progress.NewWriter()
// 	pw.SetAutoStop(autoStop)
// 	pw.SetTrackerLength(25)
// 	pw.ShowOverallTracker(true)
// 	pw.ShowTime(true)
// 	pw.ShowTracker(true)
// 	pw.ShowValue(true)
// 	pw.SetMessageWidth(24)
// 	// pw.SetNumTrackersExpected()
// 	pw.SetSortBy(progress.SortByPercentDsc)
// 	pw.SetStyle(progress.StyleDefault)
// 	pw.SetTrackerPosition(progress.PositionRight)
// 	pw.SetUpdateFrequency(time.Millisecond * 100)
// 	pw.Style().Colors = progress.StyleColorsExample
// 	pw.Style().Options.PercentFormat = "%4.1f%%"
//
// 	// call Render() in async mode; yes we don't have any trackers at the moment
// 	go pw.Render()
//
// 	// add a bunch of trackers with random parameters to demo most of the
// 	// features available; do this in async too like a client might do (for ex.
// 	// when downloading a bunch of files in parallel)
// 	for idx := int64(1); idx <= numOfImages; idx++ {
// 		go trackSomething(pw, idx, tableValues)
//
// 		// in auto-stop mode, the Render logic terminates the moment it detects
// 		// zero active trackers; but in a manual-stop mode, it keeps waiting and
// 		// is a good chance to demo trackers being added dynamically while other
// 		// trackers are active or done
// 		if !autoStop {
// 			time.Sleep(time.Millisecond * 100)
// 		}
// 	}
//
// 	// wait for one or more trackers to become active (just blind-wait for a
// 	// second) and then keep watching until Rendering is in progress
// 	time.Sleep(time.Second)
// 	for pw.IsRenderInProgress() {
// 		// for manual-stop mode, stop when there are no more active trackers
// 		if pw.LengthActive() == 0 {
// 			pw.Stop()
// 		}
// 		time.Sleep(time.Millisecond * 100)
// 	}
//
// 	fmt.Println("\nAll done!")
// 	printingFinishedChannel <- true
// 	close(printingFinishedChannel)
// }

// func trackSomething(pw progress.Writer, idx int64, tableValues ColChanStruct) {
// 	var message string
//
// 	progress.UnitsDefault
//
// 	tracker := progress.Tracker{Message: message, Total: total, Units: *units}
//
// 	pw.AppendTracker(&tracker)
//
// 	c := time.Tick(time.Millisecond * 100)
// 	for !tracker.IsDone() {
// 		select {
// 		case <-c:
// 			tracker.Increment(incrementPerCycle)
// 		}
// 	}
//
// }
