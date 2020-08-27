package bd_xray

import (
	"context"
	"fmt"

	"github.com/jedib0t/go-pretty/table"
	"github.com/oklog/run"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/blackducksoftware/kubectl-bd-xray/pkg/detect"
	"github.com/blackducksoftware/kubectl-bd-xray/pkg/util"
	"github.com/blackducksoftware/kubectl-bd-xray/pkg/kube"
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

type namespaceScanFlags struct {
	DetectOfflineMode string
	BlackDuckURL      string
	BlackDuckToken    string
	DetectProjectName string
	DetectVersionName string
	LoggingLevel      string
	// TODO: add how many scans to process simultaneously
	// ConcurrencyLevel  string
}

func SetupNamespaceScanCommand() *cobra.Command {
	namespaceScanFlags := &namespaceScanFlags{}

	flagMap := map[string]interface{}{
		DetectOfflineModeFlag: &namespaceScanFlags.DetectOfflineMode,
		BlackDuckURLFlag:      &namespaceScanFlags.BlackDuckURL,
		BlackDuckTokenFlag:    &namespaceScanFlags.BlackDuckToken,
		DetectProjectNameFlag: &namespaceScanFlags.DetectProjectName,
		DetectVersionNameFlag: &namespaceScanFlags.DetectVersionName,
	}

	ctx, cancel := context.WithCancel(context.Background())
	var goRoutineGroup run.Group
	var printerGoRoutine run.Group
	outputChan := make(chan *ScanStatusTableValues)
	printingFinishedChannel := make(chan bool, 1)

	command := &cobra.Command{
		Use:   "namespace NAMESPACE_NAME...",
		Short: "",
		Long:  "",
		Args: func(cmd *cobra.Command, args []string) error {
			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			printerGoRoutine.Add(func() error {
				PrintScanStatusTable(outputChan, printingFinishedChannel)
				return nil
			}, func(error) {
				cancel()
			})
			var err error
			var imageList []string

			cli, _ := kube.NewDefaultClient()
			imageList, err = cli.GetImagesFromNamespace(context.Background(), args[0])
			
			for _, image := range imageList {
				image := image
				log.Infof("Scanning image: %s", image)
				goRoutineGroup.Add(func() error {
					return RunImageScanCommand(ctx, image, flagMap, outputChan)
				}, func(error) {
					cancel()
				})
			}

			log.Debugf("starting printer goroutine")

			go printerGoRoutine.Run()

			log.Debugf("starting scanning goroutines")

			err = goRoutineGroup.Run()
			if err != nil {
				log.Fatalf("FATAL ERROR: %+v", err)
			}
			log.Tracef("closing the output channel")

			close(outputChan)
			// time.Sleep(10 * time.Second)

			// TODO: Block on printerGoRoutine
			select {
			case <-printingFinishedChannel:
				log.Infof("All done!")
			}
		},
	}

	command.Flags().StringVar(&namespaceScanFlags.DetectOfflineMode, DetectOfflineModeFlag, "false", "Enabled Offline Scanning")
	command.Flags().StringVar(&namespaceScanFlags.BlackDuckURL, BlackDuckURLFlag, "", "Black Duck Server URL")
	command.Flags().StringVar(&namespaceScanFlags.BlackDuckToken, BlackDuckTokenFlag, "", "Black Duck API Token")
	command.Flags().StringVar(&namespaceScanFlags.DetectProjectName, DetectProjectNameFlag, "", "An override for the name to use for the Black Duck project. If not supplied, Detect will attempt to use the tools to figure out a reasonable project name.")
	command.Flags().StringVar(&namespaceScanFlags.DetectVersionName, DetectVersionNameFlag, "", "An override for the version to use for the Black Duck project. If not supplied, Detect will attempt to use the tools to figure out a reasonable version name. If that fails, the current date will be used.")

	return command
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
	var printerGoRoutine run.Group
	outputChan := make(chan *ScanStatusTableValues)
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
				PrintScanStatusTable(outputChan, printingFinishedChannel)
				return nil
			}, func(error) {
				cancel()
			})
			var err error
			var imageList []string

			cli, _ := kube.NewDefaultClient()
			imageList, err = cli.GetImagesFromNamespace(context.Background(), "local-path-storage")

			for _, image := range imageList {
				image := image
				log.Infof("Scanning image: %s", image)
				goRoutineGroup.Add(func() error {
					return RunImageScanCommand(ctx, image, flagMap, outputChan)
				}, func(error) {
					cancel()
				})
			}

			log.Debugf("starting printer goroutine")

			go printerGoRoutine.Run()

			log.Debugf("starting scanning goroutines")

			err = goRoutineGroup.Run()
			if err != nil {
				log.Fatalf("FATAL ERROR: %+v", err)
			}
			log.Tracef("closing the output channel")

			close(outputChan)
			// time.Sleep(10 * time.Second)

			// TODO: Block on printerGoRoutine
			select {
			case <-printingFinishedChannel:
				log.Infof("All done!")
			}
		},
	}

	command.Flags().StringVar(&imageScanFlags.DetectOfflineMode, DetectOfflineModeFlag, "false", "Enabled Offline Scanning")
	command.Flags().StringVar(&imageScanFlags.BlackDuckURL, BlackDuckURLFlag, "", "Black Duck Server URL")
	command.Flags().StringVar(&imageScanFlags.BlackDuckToken, BlackDuckTokenFlag, "", "Black Duck API Token")
	command.Flags().StringVar(&imageScanFlags.DetectProjectName, DetectProjectNameFlag, "", "An override for the name to use for the Black Duck project. If not supplied, Detect will attempt to use the tools to figure out a reasonable project name.")
	command.Flags().StringVar(&imageScanFlags.DetectVersionName, DetectVersionNameFlag, "", "An override for the version to use for the Black Duck project. If not supplied, Detect will attempt to use the tools to figure out a reasonable version name. If that fails, the current date will be used.")

	return command
}



func RunImageScanCommand(ctx context.Context, fullImageName string, flagMap map[string]interface{}, scanStatusTableValues chan *ScanStatusTableValues) error {
	var err error

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

	err = detectClient.DockerCLIClient.PullDockerImage(fullImageName)
	if err != nil {
		return err
	}

	// uniqueOutputDirName := fmt.Sprintf("%s/%s_%s", detect.DefaultDetectBlackduckDirectory, image, util.GenerateRandomString(16))
	imageName := util.ParseImageName(fullImageName)
	imageTag := util.ParseImageTag(fullImageName)
	imageSha, err := detectClient.DockerCLIClient.GetImageSha(fullImageName)
	if err != nil {
		return err
	}
	// a unique string, but something that's human readable, i.e.: IMAGENAME_SHA_RANDOMSTRING(or timestamp)
	uniqueOutputDirName := fmt.Sprintf("%s/%s_%s_%s", detect.DefaultDetectBlackduckDirectory, imageName, imageSha, util.GenerateRandomString(16))
	log.Tracef("output dir is: %s", uniqueOutputDirName)
	// actually scan
	log.Tracef("starting image scan")

	err = detectClient.RunImageScan(fullImageName, imageName, imageTag, imageSha, uniqueOutputDirName, flagsToPassToDetect)
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
