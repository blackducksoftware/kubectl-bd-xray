package detect

import (
	"fmt"
	"os"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"

	"github.com/blackducksoftware/kubectl-bd-xray/pkg/docker"
	"github.com/blackducksoftware/kubectl-bd-xray/pkg/util"
)

const (
	// TODO: maybe makes sense to put detect script under ~/blackduck as well (other than the bootstrap problem)
	DefaultDetectDownloadFilePath = "./detect.sh"
	DefaultDetectURL              = "https://detect.synopsys.com/detect.sh"
	WindowsDetectURL              = "https://detect.synopsys.com/detect.ps1"
)

var (
	DefaultDetectBlackduckDirectory = fmt.Sprintf("%s/blackduck", util.GetHomeDir())
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
	util.DoOrDie(err)

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
	_, err = util.ChmodX(c.DetectPath)
	return err
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
	log.Infof("scanning: %s", fullImageName)

	// a unique string, but something that's human readable, i.e.: NAME_TAG
	uniqueSanitizedString := util.SanitizeString(fmt.Sprintf("%s_%s", imageName, imageTag))

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

	var cmdStr string
	cmdStr = fmt.Sprintf("%s %s %s %s %s %s", c.DetectPath, defaultGlobalFlags, userSpecifiedDetectFlags, c.GetProjectNameFlag(projectName), c.GetProjectVersionNameFlag(projectVersionName), c.GetPersistentDockerInspectorServicesFlags())
	cmdStr += fmt.Sprintf(" %s", c.GetDetectDefaultScanFlags(fullImageName))
	// cmdStr = fmt.Sprintf(" %s %s", c.GetConcurrentDockerInspectorScanFlags(), c.GetAllUnsquashedScanFlags(unsquashedImageTarFilePath))
	// cmdStr = fmt.Sprintf(" %s", c.GetAllSquashedScanFlags(squashedImageTarFilePath, fullImageName))
	cmd := util.GetExecCommandFromString(cmdStr)

	// NOTE: by design, we explicitly don't print out the detect output
	err = util.RunCommandBasedOnLoggingLevel(cmd)
	return err
}

// GetDetectDefaultScanFlags: this is the default scan that detect invokes (which is just docker-inspector + squashed signature scanner)
func (c *Client) GetDetectDefaultScanFlags(fullImageName string) string {
	return fmt.Sprintf("--detect.docker.image=%s", fullImageName)
}

// GetPersistentDockerInspectorServicesFlags: flags to pass to detect if docker-inspector is setup to run on host with
// each image inspector service runs in a container
// https://blackducksoftware.github.io/blackduck-docker-inspector/latest/deployment/#deployment-sample-for-docker-using-persistent-image-inspector-services
// https://github.com/blackducksoftware/blackduck-docker-inspector/blob/9.1.1/deployment/docker/runDetectAgainstDockerServices/setup.sh#L111
// https://synopsys.atlassian.net/wiki/spaces/INTDOCS/pages/760021042/Docker+Inspector+Properties
func (c *Client) GetPersistentDockerInspectorServicesFlags() string {
	return fmt.Sprintf("--detect.docker.path.required=false --detect.docker.passthrough.imageinspector.service.url=http://localhost:9002 --detect.docker.passthrough.imageinspector.service.start=false --detect.docker.passthrough.shared.dir.path.local=%s/blackduck/shared", util.GetHomeDir())
}

// SetupPersistentDockerInspectorServices: sets up persistent docker services on host; goes together with GetPersistentDockerInspectorServicesFlags
func (c *Client) SetupPersistentDockerInspectorServices() error {
	// first setup docker-inspector
	cmd := util.GetExecCommandFromString(fmt.Sprintf("sh -c ../../pkg/detect/runDetectAgainstDockerServices.sh"))
	return util.RunCommandBasedOnLoggingLevel(cmd)
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
	return fmt.Sprintf("--detect.tools=DOCKER %s", c.GetDetectDefaultScanFlags(fullImageName))
}

// GetSignatureScanOnlyFlags signature scanner only
func (c *Client) GetSignatureScanOnlyFlags(imageTarFilePath string) string {
	// NOTE: project name is required, since otherwise it uses the directory name from which the tarball was scanned
	return fmt.Sprintf("--detect.tools=SIGNATURE_SCAN --detect.blackduck.signature.scanner.paths=%s", imageTarFilePath)
}

// GetBinaryScanOnlyFlags binary scanner only
func (c *Client) GetBinaryScanOnlyFlags(imageTarFilePath string) string {
	// TODO: not sure if project name is required
	return fmt.Sprintf("--detect.tools=BINARY_SCAN --detect.binary.scan.file.path=%s", imageTarFilePath)
}

// GetAllSquashedScanFlags [squashed] docker-inspector + signature + binary
func (c *Client) GetAllSquashedScanFlags(squashedImageTarFilePath, fullImageName string) string {
	return fmt.Sprintf("--detect.tools=DOCKER,SIGNATURE_SCAN,BINARY_SCAN --detect.docker.image=%s --detect.binary.scan.file.path=%s", fullImageName, squashedImageTarFilePath)
}

// GetAllUnsquashedScanFlags [unsquashed] docker-inspector + signature + binary
func (c *Client) GetAllUnsquashedScanFlags(unsquashedImageTarFilePath string) string {
	// TODO: not sure if project name is required, since docker-inspector is supposed to auto-fill that
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
