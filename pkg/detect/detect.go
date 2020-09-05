package detect

import (
	"fmt"
	"os"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"

	"github.com/blackducksoftware/kubectl-bd-xray/pkg/docker"
	"github.com/blackducksoftware/kubectl-bd-xray/pkg/utils"
)

const (
	// TODO: maybe makes sense to put detect script under ~/blackduck as well (other than the bootstrap problem)
	DefaultDetectDownloadFilePath = "./detect.sh"
	DefaultDetectURL              = "https://detect.synopsys.com/detect.sh"
	WindowsDetectURL              = "https://detect.synopsys.com/detect.ps1"
	// Modified from here: https://github.com/blackducksoftware/blackduck-docker-inspector/blob/9.1.1/deployment/docker/runDetectAgainstDockerServices/setup.sh
	// TODO: keep sync'd to runDetectAgainstDockerServices.sh and/or delete that bash script
	RunDetectAgainstDockerServicesBashScript = `
#!/bin/bash

####################################################################
# This script demonstrates how you can use detect / docker inspector
# such that the image inspector services get started once, and
# re-used by each subsequent run of Detect / Docker inspector.
# This results in much faster performance since Detect / Docker
# Inspector doesn't start/stop the services every time.
####################################################################

####################################################################
# ==> Adjust the value of localSharedDirPath
# This path cannot have any symbolic links in it
####################################################################
localSharedDirPath=~/blackduck/shared
imageInspectorVersion=5.0.1

####################################################################
# This script will start the imageinspector alpine service (if
# it's not already running).
#
# This script will leave the alpine imageinspector service running
# (and will re-use it on subsequent runs)
# For troubleshooting, you might need to do a "docker logs" on
# the imageinspector service container.
#
# All three "docker run" commands (starting the services) will have
# all 3 port numbers in them, so every service knows how to find
# every other service.
# This allows a service to redirect requests to the other services when necessary.
####################################################################

mkdir -p ${localSharedDirPath}/target

# Make sure the alpine service is running

alpineServiceIsUp=false

successMsgCount=$(curl http://localhost:9000/health | grep "\"status\":\"UP\"" | wc -l)
if [ "$successMsgCount" -eq "1" ]; then
	echo "The alpine image inspector service is up"
	alpineServiceIsUp=true
else
	# Start the image inspector service for alpine on port 9000
	docker run -d --user 1001 -p 9000:8081 --label "app=blackduck-imageinspector" --label="os=ALPINE" -v ${localSharedDirPath}:/opt/blackduck/blackduck-imageinspector/shared --name blackduck-imageinspector-alpine blackducksoftware/blackduck-imageinspector-alpine:${imageInspectorVersion} java -jar /opt/blackduck/blackduck-imageinspector/blackduck-imageinspector.jar --server.port=8081 --current.linux.distro=alpine --inspector.url.alpine=http://localhost:9000 --inspector.url.centos=http://localhost:9001 --inspector.url.ubuntu=http://localhost:9002
fi

while [ "$alpineServiceIsUp" == "false" ]; do
	successMsgCount=$(curl http://localhost:9000/health | grep "\"status\":\"UP\"" | wc -l)
	if [ "$successMsgCount" -eq "1" ]; then
		echo "The alpine service is up"
		alpineServiceIsUp=true
		break
	fi
	echo "The alpine service is not up yet"
	sleep 15
done

# Make sure the centos service is running

centosServiceIsUp=false

successMsgCount=$(curl http://localhost:9001/health | grep "\"status\":\"UP\"" | wc -l)
if [ "$successMsgCount" -eq "1" ]; then
	echo "The centos image inspector service is up"
	centosServiceIsUp=true
else
	# Start the image inspector service for centos on port 9001
	docker run -d --user 1001 -p 9001:8081 --label "app=blackduck-imageinspector" --label="os=CENTOS" -v ${localSharedDirPath}:/opt/blackduck/blackduck-imageinspector/shared --name blackduck-imageinspector-centos blackducksoftware/blackduck-imageinspector-centos:${imageInspectorVersion} java -jar /opt/blackduck/blackduck-imageinspector/blackduck-imageinspector.jar --server.port=8081 --current.linux.distro=centos --inspector.url.alpine=http://localhost:9000 --inspector.url.centos=http://localhost:9001 --inspector.url.ubuntu=http://localhost:9002
fi

while [ "$centosServiceIsUp" == "false" ]; do
	successMsgCount=$(curl http://localhost:9001/health | grep "\"status\":\"UP\"" | wc -l)
	if [ "$successMsgCount" -eq "1" ]; then
		echo "The centos service is up"
		centosServiceIsUp=true
		break
	fi
	echo "The centos service is not up yet"
	sleep 15
done

# Make sure the ubuntu service is running

ubuntuServiceIsUp=false

successMsgCount=$(curl http://localhost:9002/health | grep "\"status\":\"UP\"" | wc -l)
if [ "$successMsgCount" -eq "1" ]; then
	echo "The ubuntu image inspector service is up"
	ubuntuServiceIsUp=true
else
	# Start the image inspector service for ubuntu on port 9002
	docker run -d --user 1001 -p 9002:8081 --label "app=blackduck-imageinspector" --label="os=UBUNTU" -v ${localSharedDirPath}:/opt/blackduck/blackduck-imageinspector/shared --name blackduck-imageinspector-ubuntu blackducksoftware/blackduck-imageinspector-ubuntu:${imageInspectorVersion} java -jar /opt/blackduck/blackduck-imageinspector/blackduck-imageinspector.jar --server.port=8081 --current.linux.distro=ubuntu --inspector.url.alpine=http://localhost:9000 --inspector.url.centos=http://localhost:9001 --inspector.url.ubuntu=http://localhost:9002
fi

while [ "$ubuntuServiceIsUp" == "false" ]; do
	successMsgCount=$(curl http://localhost:9002/health | grep "\"status\":\"UP\"" | wc -l)
	if [ "$successMsgCount" -eq "1" ]; then
		echo "The ubuntu service is up"
		ubuntuServiceIsUp=true
		break
	fi
	echo "The ubuntu service is not up yet"
	sleep 15
done

# cd run
# curl -O https://detect.synopsys.com/detect.sh
# chmod +x detect.sh

# ./detect.sh --blackduck.offline.mode=true --detect.tools.excluded=SIGNATURE_SCAN,POLARIS --detect.docker.image=alpine:latest --detect.docker.path.required=false --detect.docker.passthrough.imageinspector.service.url=http://localhost:9002 --detect.docker.passthrough.imageinspector.service.start=false --logging.level.com.synopsys.integration=INFO --detect.docker.passthrough.shared.dir.path.local=${localSharedDirPath}
`
)

var (
	DefaultDetectBlackduckDirectory = fmt.Sprintf("%s/blackduck", utils.GetHomeDir())
	DefaultToolsDirectory           = fmt.Sprintf("%s/tools", DefaultDetectBlackduckDirectory)
)

type Client struct {
	DetectPath      string
	DetectURL       string
	RestyClient     *resty.Client
	DockerCLIClient *docker.DockerCLIClient
}

func NewDefaultClient() *Client {
	return NewClient(
		DefaultDetectDownloadFilePath,
		DefaultDetectURL)
}

func NewClient(detectFilePath, detectURL string) *Client {
	restyClient := resty.New().
		// for all requests, you can use relative path; resty doesn't care whether relative path starts with /
		SetHostURL(detectURL).
		// exponential backoff: https://github.com/go-resty/resty#retries
		SetRetryCount(3).
		// set this to true if you want to get more info, including response metrics
		SetDebug(false).
		SetTimeout(180 * time.Second)

	dockerCLIClient, err := docker.NewCliClient()
	utils.DoOrDie(err)

	return &Client{
		DetectPath:      detectFilePath,
		DetectURL:       detectURL,
		RestyClient:     restyClient,
		DockerCLIClient: dockerCLIClient,
	}
}

func (c *Client) DownloadDetect() error {
	request := c.RestyClient.R().
		// see: https://github.com/go-resty/resty#save-http-response-into-file
		SetOutput(c.DetectPath)
	resp, err := request.Get(c.DetectURL)
	if err != nil {
		return errors.Wrapf(err, "issue GET request to %s", request.URL)
	}
	respBody, statusCode := resp.String(), resp.StatusCode()
	if !resp.IsSuccess() {
		return errors.Errorf("bad status code to path %s: %d, response %s", c.DetectURL, statusCode, respBody)
	}
	return os.Chmod(c.DetectPath, 0755)
}

func (c *Client) DownloadDetectIfNotExists() error {
	if _, err := os.Stat(c.DetectPath); err == nil {
		log.Debugf("detect found at %s, not downloading again, running sync recommended", c.DetectPath)
		// TODO: sync to latest version of the specified Black Duck server, if possible
		return nil
	} else if os.IsNotExist(err) {
		log.Debugf("detect not found at %s, downloading ...", c.DetectPath)
		// if detect not found at path, then download a fresh copy
		return c.DownloadDetect()
	} else {
		return errors.Wrapf(err, "unable to check if file %s exists", c.DetectPath)
	}
}

func (c *Client) RunImageScan(fullImageName, projectName, imageName, imageTag, outputDirName, userSpecifiedDetectFlags string) error {
	var err error
	log.Infof("scanning: '%s'", fullImageName)

	// a unique string, but something that's human readable, i.e.: NAME_TAG
	uniqueSanitizedString := utils.SanitizeString(fmt.Sprintf("%s_%s", imageName, imageTag))

	// UNSQUASHED
	// unsquashedImageTarFilePath := fmt.Sprintf("unsquashed_%s.tar", uniqueSanitizedString)
	// log.Tracef("unsquashed image tar file path: %s", unsquashedImageTarFilePath)
	// c.DockerCLIClient.SaveDockerImage(imageName, unsquashedImageTarFilePath)

	// SQUASHED
	// squashedImageTarFilePath := fmt.Sprintf("squashed_%s.tar", uniqueSanitizedString)
	// log.Tracef("squashed image tar file path: %s", squashedImageTarFilePath)
	// err = dockersquash.DockerSquash(fullImageName, squashedImageTarFilePath)
	// if err != nil {
	// 	return err
	// }

	// TODO: according to docs here: https://synopsys.atlassian.net/wiki/spaces/INTDOCS/pages/650969090/Diagnostic+Mode --diagnosticExtended flag means logging is set to debug and cleanup is set to false by default, however, it seems --detect.cleanup=false is needed in order to keep the status.json file.
	// --diagnosticExtended
	// --logging.level.com.synopsys.integration=OFF
	// --detect.cleanup=false
	defaultGlobalFlags := fmt.Sprintf("--detect.cleanup=false --blackduck.trust.cert=true --detect.tools.output.path=%s --detect.output.path=%s", DefaultToolsDirectory, outputDirName)
	log.Tracef("default global flags: %s", defaultGlobalFlags)

	var projectVersionName string
	if 0 == len(projectName) {
		projectName = imageName
		projectVersionName = imageTag
	} else {
		projectVersionName = uniqueSanitizedString
	}
	codeLocationName := projectName

	var cmdStr string
	cmdStr = fmt.Sprintf("%s %s %s %s %s %s %s", c.DetectPath, defaultGlobalFlags, userSpecifiedDetectFlags, c.GetProjectNameFlag(projectName), c.GetProjectVersionNameFlag(projectVersionName), c.GetCodeLocationNameFlag(codeLocationName), c.GetPersistentDockerInspectorServicesFlags())
	cmdStr += fmt.Sprintf(" %s", c.GetDockerInspectorAndSignatureOnlyScanFlags(fullImageName))
	// cmdStr += fmt.Sprintf(" %s", c.GetDockerInspectorScanOnlyFlags(fullImageName))
	// cmdStr = fmt.Sprintf(" %s", c.GetAllSquashedScanFlags(squashedImageTarFilePath, fullImageName))
	cmd := utils.GetExecCommandFromString(cmdStr)

	// NOTE: by design, we explicitly don't print out the detect output
	err = utils.RunCommandBasedOnLoggingLevel(cmd)
	return err
}

// GetDetectDockerImageDefaultScanFlags: this is the default scan that detect invokes (which is just docker-inspector + squashed signature scanner)
func (c *Client) GetDetectDockerImageDefaultScanFlags(fullImageName string) string {
	return fmt.Sprintf("--detect.docker.image=%s --detect.tools.excluded=DETECTOR,POLARIS", fullImageName)
}

// GetDockerInspectorAndSignatureOnlyScanFlags: explicitly only run DOCKER,SIGNATURE_SCAN scans for specified image
func (c *Client) GetDockerInspectorAndSignatureOnlyScanFlags(fullImageName string) string {
	return fmt.Sprintf("--detect.tools=DOCKER,SIGNATURE_SCAN %s", c.GetDetectDockerImageDefaultScanFlags(fullImageName))
}

// GetPersistentDockerInspectorServicesFlags: flags to pass to detect if docker-inspector is setup to run on host with
// each image inspector service runs in a container
// https://blackducksoftware.github.io/blackduck-docker-inspector/latest/deployment/#deployment-sample-for-docker-using-persistent-image-inspector-services
// https://github.com/blackducksoftware/blackduck-docker-inspector/blob/9.1.1/deployment/docker/runDetectAgainstDockerServices/setup.sh#L111
// https://synopsys.atlassian.net/wiki/spaces/INTDOCS/pages/760021042/Docker+Inspector+Properties
func (c *Client) GetPersistentDockerInspectorServicesFlags() string {
	return fmt.Sprintf("--detect.docker.path.required=false --detect.docker.passthrough.imageinspector.service.url=http://localhost:9002 --detect.docker.passthrough.imageinspector.service.start=false --detect.docker.passthrough.shared.dir.path.local=%s/blackduck/shared", utils.GetHomeDir())
}

// SetupPersistentDockerInspectorServices: sets up persistent docker services on host; goes together with GetPersistentDockerInspectorServicesFlags
// https://github.com/blackducksoftware/blackduck-docker-inspector/blob/9.1.1/deployment/docker/runDetectAgainstDockerServices/setup.sh
func (c *Client) SetupPersistentDockerInspectorServices() error {
	var err error
	// first setup docker-inspector
	succeeded := utils.RunBash("set up persistent docker inspector services for concurrent scanning", RunDetectAgainstDockerServicesBashScript)
	if !succeeded {
		return errors.Errorf("error running the runDetectAgainstDockerServices script directly from golang")
	}
	// cmd := utils.GetExecCommandFromString(fmt.Sprintf("sh -c ../../pkg/detect/runDetectAgainstDockerServices.sh"))
	// err = utils.RunCommandBasedOnLoggingLevel(cmd)
	return err
}

func (c *Client) StopAndCleanupPersistentDockerInspectorServices() error {
	var err error

	succeeded := utils.RunBash("StopAndCleanupPersistentDockerInspectorServices", "docker stop blackduck-imageinspector-ubuntu blackduck-imageinspector-centos blackduck-imageinspector-alpine && docker rm blackduck-imageinspector-ubuntu blackduck-imageinspector-centos blackduck-imageinspector-alpine")
	if !succeeded {
		return errors.Errorf("error stopping and removing persistent docker inspector service containers")
	}

	return err
}

// GetConcurrentDockerInspectorScanFlags: ask inspector not to cleanup services it spins up to re-use;
// must wait a bit after first scan and then run concurrently
// TODO: not working, check application properties to see if flag actually gets passed
//  https://synopsys.atlassian.net/wiki/spaces/INTDOCS/pages/760381459/Concurrent+Execution
func (c *Client) GetConcurrentDockerInspectorScanFlags() string {
	// passthrough flag makes docker-inspector be able to run multiple scans
	return fmt.Sprintf("--detect.docker.passthrough.imageinspector.cleanup.inspector.container=false")
}

// GetDockerInspectorScanOnlyFlags docker-inspector only
func (c *Client) GetDockerInspectorScanOnlyFlags(fullImageName string) string {
	return fmt.Sprintf("--detect.tools=DOCKER %s", c.GetDetectDockerImageDefaultScanFlags(fullImageName))
}

// GetSignatureScanOnlyFlags signature scanner only
func (c *Client) GetSignatureScanOnlyFlags(imageTarFilePath string) string {
	// NOTE: project name is required, since otherwise it uses the directory name from which the tarball was scanned
	return fmt.Sprintf("--detect.tools=SIGNATURE_SCAN --detect.blackduck.signature.scanner.paths=%s", imageTarFilePath)
}

// GetBinaryScanOnlyFlags binary scanner only
func (c *Client) GetBinaryScanOnlyFlags(imageTarFilePath string) string {
	return fmt.Sprintf("--detect.tools=BINARY_SCAN --detect.binary.scan.file.path=%s", imageTarFilePath)
}

// GetAllSquashedScanFlags [squashed] docker-inspector + signature + binary
func (c *Client) GetAllSquashedScanFlags(squashedImageTarFilePath, fullImageName string) string {
	return fmt.Sprintf("--detect.tools=DOCKER,SIGNATURE_SCAN,BINARY_SCAN --detect.docker.image=%s --detect.binary.scan.file.path=%s", fullImageName, squashedImageTarFilePath)
}

// GetAllUnsquashedScanFlags [unsquashed] docker-inspector + signature + binary
func (c *Client) GetAllUnsquashedScanFlags(unsquashedImageTarFilePath string) string {
	return fmt.Sprintf("--detect.tools=DOCKER,SIGNATURE_SCAN,BINARY_SCAN --detect.docker.tar=%s --detect.binary.scan.file.path=%s", unsquashedImageTarFilePath, unsquashedImageTarFilePath)
}

// GetBinaryAndSignatureScanFlags signature + binary scanners
func (c *Client) GetBinaryAndSignatureScanFlags(imageTarFilePath string) string {
	return fmt.Sprintf("--detect.tools=SIGNATURE_SCAN,BINARY_SCAN --detect.blackduck.signature.scanner.paths=%s --detect.binary.scan.file.path=%s", imageTarFilePath, imageTarFilePath)
}

// GetProjectNameFlag sets up the project name flag
func (c *Client) GetProjectNameFlag(projectName string) string {
	return fmt.Sprintf("--detect.project.name=%s", projectName)
}

// GetProjectVersionNameFlag sets up the project version name flag
func (c *Client) GetProjectVersionNameFlag(projectVersionName string) string {
	return fmt.Sprintf("--detect.project.version.name=%s", projectVersionName)
}

// GetCodeLocationNameFlag sets up the code location name flag
func (c *Client) GetCodeLocationNameFlag(codeLocationName string) string {
	return fmt.Sprintf("--detect.code.location.name=%s", codeLocationName)
}
