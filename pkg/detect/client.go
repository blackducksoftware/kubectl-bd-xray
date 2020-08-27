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

func (c *Client) RunImageScan(fullImageName, imageName, imageTag, imageSha, outputDirName, userSpecifiedDetectFlags string) error {

	var err error
	// a unique string, but something that's human readable, i.e.: IMAGENAME_TAG_SHA
	uniqueString := fmt.Sprintf("%s_%s_%s", imageName, imageTag, imageSha)
	unsquashedImageTarFilePath := fmt.Sprintf("unsquashed_%s.tar", uniqueString)
	squashedImageTarFilePath := fmt.Sprintf("squashed_%s.tar", uniqueString)
	log.Tracef("unsquashed image tar file path: %s", unsquashedImageTarFilePath)
	log.Tracef("squashed image tar file path: %s", squashedImageTarFilePath)

	// UNSQUASHED
	// c.DockerCLIClient.SaveDockerImage(imageName, unsquashedImageTarFilePath)

	// SQUASHED
	log.Infof("full image scan: %s", fullImageName)
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
	// TODO: figure out concurrent docker-inspector scans
	cmd := util.GetExecCommandFromString(fmt.Sprintf("%s %s %s %s %s", c.DetectPath, c.GetDetectDefaultScanFlags(imageName), c.GetConcurrentDockerInspectorScanFlags(), defaultGlobalFlags, userSpecifiedDetectFlags))
	// cmd := util.GetExecCommandFromString(fmt.Sprintf("%s %s %s %s", c.DetectPath, c.GetSignatureScanOnlyFlags(unsquashedImageTarFilePath, imageName, ""), defaultGlobalFlags, userSpecifiedDetectFlags))
	// cmd := util.GetExecCommandFromString(fmt.Sprintf("%s %s %s %s", c.DetectPath, c.GetBinaryScanOnlyFlags(unsquashedImageTarFilePath, imageName, ""), defaultGlobalFlags, userSpecifiedDetectFlags))
	// cmd := util.GetExecCommandFromString(fmt.Sprintf("%s %s %s %s", c.DetectPath, c.GetAllConcurrentUnsquashedScanFlags(unsquashedImageTarFilePath, imageName, ""), defaultGlobalFlags, userSpecifiedDetectFlags))
	// cmd := util.GetExecCommandFromString(fmt.Sprintf("%s %s %s %s", c.DetectPath, c.GetAllConcurrentSquashedScanFlags(squashedImageTarFilePath, fullImageName, imageName, imageTag, imageSha), defaultGlobalFlags, userSpecifiedDetectFlags))
	log.Tracef("command is: %s", cmd)

	// NOTE: by design, we explicitly don't print out the detect output
	// TODO: add a column in table for where detect logs so users can examine afterwards if needed
	if log.GetLevel() == log.TraceLevel {
		// if trace enabled, allow capturing progress
		log.Tracef("since trace level is enabled, will capture progress as command executes")
		err = util.RunCommandAndCaptureProgress(cmd)
	} else {
		log.Tracef("output will be printed at the end")
		// otherwise, just print at the end
		_, err = util.RunCommand(cmd)
	}
	return err
}

// GetDetectDefaultScanFlags: this is the default scan that detect invokes (which is just docker-inspector + signature scanner)
func (c *Client) GetDetectDefaultScanFlags(imageName string) string {
	return fmt.Sprintf("--detect.docker.image=%s", imageName)
}

func (c *Client) GetConcurrentDockerInspectorScanFlags() string {
	// passthrough flag makes docker-inspector be able to run multiple scans
	return fmt.Sprintf("--detect.docker.passthrough.imageinspector.cleanup.inspector.container=false")
}

// GetDockerInspectorScanOnlyFlags docker-inspector only
func (c *Client) GetDockerInspectorScanOnlyFlags(imageName string) string {
	return fmt.Sprintf("--detect.tools=DOCKER %s", c.GetDetectDefaultScanFlags(imageName))
}

// GetSignatureScanOnlyFlags signature scanner only
func (c *Client) GetSignatureScanOnlyFlags(imageTarFilePath, imageName, imageVersion string) string {
	// NOTE: project name is required, since otherwise it uses the directory name from which the tarball was scanned
	return fmt.Sprintf("--detect.tools=SIGNATURE_SCAN --detect.blackduck.signature.scanner.paths=%s %s", imageTarFilePath, c.GetProjectNameFlag(imageName))
}

// GetBinaryScanOnlyFlags binary scanner only
func (c *Client) GetBinaryScanOnlyFlags(imageTarFilePath, imageName, imageVersion string) string {
	// TODO: not sure if project name is required
	return fmt.Sprintf("--detect.tools=BINARY_SCAN --detect.binary.scan.file.path=%s %s", imageTarFilePath, c.GetProjectNameFlag(imageName))
}

// GetBinaryAndSignatureScanFlags signature + binary scanners
func (c *Client) GetBinaryAndSignatureScanFlags(imageTarFilePath, imageName, imageVersion string) string {
	return fmt.Sprintf("--detect.tools=SIGNATURE_SCAN,BINARY_SCAN --detect.blackduck.signature.scanner.paths=%s --detect.binary.scan.file.path=%s %s", imageTarFilePath, imageTarFilePath, c.GetProjectNameFlag(imageName))
}

// GetAllScanFlags docker-inspector + signature + binary
func (c *Client) GetAllScanFlags(imageTarFilePath, imageName, imageVersion string) string {
	// TODO: not sure if project name is required, since docker-inspector is supposed to auto-fill that
	return fmt.Sprintf("--detect.tools=DOCKER,SIGNATURE_SCAN,BINARY_SCAN --detect.docker.tar=%s --detect.binary.scan.file.path=%s %s", imageTarFilePath, imageTarFilePath, c.GetProjectNameFlag(imageName))
}

// GetAllConcurrentUnsquashedScanFlags docker-inspector + signature + binary
func (c *Client) GetAllConcurrentUnsquashedScanFlags(imageTarFilePath, imageName, imageVersion string) string {
	// TODO: not sure if project name is required, since docker-inspector is supposed to auto-fill that
	return fmt.Sprintf("%s --detect.tools=DOCKER,SIGNATURE_SCAN,BINARY_SCAN --detect.docker.tar=%s --detect.binary.scan.file.path=%s %s", c.GetConcurrentDockerInspectorScanFlags(), imageTarFilePath, imageTarFilePath, c.GetProjectNameFlag(imageName))
}

func (c *Client) GetAllConcurrentSquashedScanFlags(squashedImageTarFilePath, fullImageName, imageName, imageTag, imageSha string) string {
	return fmt.Sprintf("%s --detect.tools=DOCKER,SIGNATURE_SCAN,BINARY_SCAN --detect.docker.image=%s --detect.binary.scan.file.path=%s %s %s", c.GetConcurrentDockerInspectorScanFlags(), fullImageName, squashedImageTarFilePath, c.GetProjectNameFlag(imageName), c.GetProjectVersionNameFlag(imageTag))
}

func (c *Client) GetProjectNameFlag(projectName string) string {
	return fmt.Sprintf("--detect.project.name=%s", projectName)
}

func (c *Client) GetProjectVersionNameFlag(projectVersionName string) string {
	return fmt.Sprintf("--detect.project.version.name=%s", projectVersionName)
}

func (c *Client) GetCodeLocationNameFlag(codeLocationName string) string {
	return fmt.Sprintf("--detect.code.location.name=%s", codeLocationName)
}
