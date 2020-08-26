package detect

import (
	"fmt"
	"os"
	"time"

	"github.com/blackducksoftware/kubectl-bd-xray/pkg/docker"
	"github.com/blackducksoftware/kubectl-bd-xray/pkg/util"
	"github.com/go-resty/resty/v2"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

const (
	DefaultDetectDownloadFilePath = "./detect.sh"
	DefaultDetectURL              = "https://detect.synopsys.com/detect.sh"
	WindowsDetectURL              = "https://detect.synopsys.com/detect.ps1"
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

func (c *Client) RunImageScan(imageName, flags string) error {

	fileName := fmt.Sprintf("unsquashed-%s.tar", imageName)
	c.DockerCLIClient.SaveDockerImage(imageName, fileName)

	// TODO: name it uniquely, best candidate is sha of the image in the folder name
	defaultFlags := fmt.Sprintf("-de --blackduck.trust.cert=true --detect.cleanup=false --detect.tools.output.path=$HOME/blackduck/tools --detect.output.path=$HOME/blackduck/%s", util.GenerateRandomString(16))
	// cmd := util.GetExecCommandFromString(fmt.Sprintf("%s %s %s --detect.docker.image=%s", c.DetectPath, defaultFlags, flags, imageName))
	cmd := util.GetExecCommandFromString(fmt.Sprintf("%s %s %s --detect.tools=SIGNATURE_SCAN --detect.blackduck.signature.scanner.paths=%s", c.DetectPath, defaultFlags, flags, fileName))
	return util.RunAndCaptureProgress(cmd)
}
