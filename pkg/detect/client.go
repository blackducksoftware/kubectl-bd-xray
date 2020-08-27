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

	dockerCLIClient, _ := docker.NewCliClient()

	return &Client{
		DetectPath:      DefaultDetectDownloadFilePath,
		DetectURL:       DefaultDetectURL,
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

	// sync to the same version as the blackduck server
}

func (c *Client) DownloadDetectIfNotExists() error {

	if _, err := os.Stat(c.DetectPath); err == nil {
		log.Debugf("detect found at %s, not downloading again, running sync recommended", c.DetectPath)
		return nil
	} else if os.IsNotExist(err) {
		log.Debugf("detect not found at %s, downloading ...", c.DetectPath)
		// if fly not found at path, then download a fresh copy
		return c.DownloadDetect()
	} else {
		return errors.Wrapf(err, "unable to check if file %s exists", c.DetectPath)
	}

}

func (c *Client) RunImageScan(imageName, outputDirName, userSpecifiedDetectFlags string) error {
	imageTarFilePath := fmt.Sprintf("unsquashed-%s.tar", imageName)
	c.DockerCLIClient.SaveDockerImage(imageName, imageTarFilePath)

	// TODO: according to docs here: https://synopsys.atlassian.net/wiki/spaces/INTDOCS/pages/650969090/Diagnostic+Mode --diagnosticExtended flag means logging is set to debug and cleanup is set to false by default, however, it seems --detect.cleanup=false is needed in order to keep the status.json file.
	// --logging.level.com.synopsys.integration=OFF
	// --detect.cleanup=false
	defaultGlobalFlags := fmt.Sprintf("--diagnosticExtended --detect.cleanup=false --blackduck.trust.cert=true --detect.tools.output.path=%s --detect.output.path=%s", DefaultToolsDirectory, outputDirName)
	// TODO: figure out concurrent docker-inspector scans
	cmd := util.GetExecCommandFromString(fmt.Sprintf("%s %s %s %s", c.DetectPath, c.GetSignatureScanOnlyFlags(imageName, imageTarFilePath, ""), defaultGlobalFlags, userSpecifiedDetectFlags))
	var err error
	// NOTE: by design, we explicitly don't print out the detect output
	// TODO: add a column in table for where detect logs so users can examine afterwards if needed
	// err = util.RunCommandAndCaptureProgress(cmd)
	_, err = util.RunCommand(cmd)
	return err
}

func (c *Client) GetDetectDefaultDockerImageScanFlags(imageName string) string {
	return fmt.Sprintf("--detect.docker.image=%s", imageName)
}

func (c *Client) GetSignatureScanOnlyFlags(imageName, imageTarFilePath, imageVersion string) string {
	return fmt.Sprintf("--detect.tools=SIGNATURE_SCAN --detect.blackduck.signature.scanner.paths=%s --detect.project.name=%s", imageTarFilePath, imageName)
}

// TODO: signature + binary scanners

// TODO: signature + binary + docker inspector scanners
