package api

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

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
	DetectPath  string
	DetectURL   string
	RestyClient *resty.Client
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

	return &Client{
		DetectPath:  DefaultDetectDownloadFilePath,
		DetectURL:   DefaultDetectURL,
		RestyClient: restyClient,
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
	_, err = ChmodX(c.DetectPath)
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

// func (c *Client) RunOfflineImageScan(image string) error {
// 	// ./detect.sh -de --logging.level.com.synopsys.integration="TRACE" \
// 	//               --blackduck.offline.mode=true \
// 	//               --detect.docker.image="gcr.io/distroless/java-debian9:11"

// 	cmd := GetExecCommandFromString(fmt.Sprintf("%s -de --logging.level.com.synopsys.integration=TRACE --detect.docker.image=%s --blackduck.offline.mode=true", c.DetectPath, image))
// 	return RunAndCaptureProgress(cmd)
// }

// func (c *Client) RunDefaultOnlineImageScan(image, blackDuckUrl, blackDuckApiToken string) error {
// 	cmd := GetExecCommandFromString(fmt.Sprintf("%s -de --logging.level.com.synopsys.integration=TRACE --detect.docker.image=%s --blackduck.url=%s --blackduck.api.token=%s --blackduck.trust.cert=true", c.DetectPath, image, blackDuckUrl, blackDuckApiToken))
// 	return RunAndCaptureProgress(cmd)
// }

func (c *Client) RunImageScan(image string, flags string) error {
	defaultFlags := fmt.Sprintf("-de --blackduck.trust.cert=true")
	cmd := GetExecCommandFromString(fmt.Sprintf("%s %s %s --detect.docker.image=%s", c.DetectPath, defaultFlags, flags, image))
	return RunAndCaptureProgress(cmd)
}

// ChmodX executes chmod +x on given filepath
func ChmodX(filePath string) (string, error) {
	cmd := GetExecCommandFromString(fmt.Sprintf("chmod +x %s", filePath))
	return RunCommand(cmd)
}

func GetExecCommandFromString(fullCmd string) *exec.Cmd {
	cmd := strings.Fields(fullCmd)
	cmdName := cmd[0]
	cmdArgs := cmd[1:]
	return exec.Command(cmdName, cmdArgs...)
}

func RunCommand(cmd *exec.Cmd) (string, error) {
	currDirectory := cmd.Dir
	if 0 == len(currDirectory) {
		currDirectory, _ = os.Executable()
	}

	log.Infof("running command: '%s' in directory: '%s'", cmd.String(), currDirectory)
	cmdOutput, err := cmd.CombinedOutput()
	cmdOutputStr := string(cmdOutput)
	log.Tracef("command: '%s' output:\n%s", cmd.String(), cmdOutput)
	return cmdOutputStr, errors.Wrapf(err, "unable to run command '%s': %s", cmd.String(), cmdOutputStr)
}

// RunAndCaptureProgress runs a long running command and continuously streams its output
func RunAndCaptureProgress(cmd *exec.Cmd) error {
	var stdoutBuf, stderrBuf bytes.Buffer
	stdoutIn, _ := cmd.StdoutPipe()
	stderrIn, _ := cmd.StderrPipe()
	// TODO: not sure why but this is needed, otherwise stdin is constantly fed input
	_, _ = cmd.StdinPipe()

	var errStdout, errStderr error
	stdout := io.MultiWriter(os.Stdout, &stdoutBuf)
	stderr := io.MultiWriter(os.Stderr, &stderrBuf)

	err := cmd.Start()
	if err != nil {
		return errors.Wrapf(err, "cmd.Start() failed for %s", cmd.String())
	}

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		_, errStdout = io.Copy(stdout, stdoutIn)
		wg.Done()
	}()

	_, errStderr = io.Copy(stderr, stderrIn)
	wg.Wait()

	err = cmd.Wait()
	if err != nil {
		return errors.Wrapf(err, "cmd.Wait() failed for %s", cmd.String())
	}

	if errStdout != nil || errStderr != nil {
		return errors.Errorf("failed to capture stdout or stderr from command '%s'", cmd.String())
	}
	// outStr, errStr := string(stdoutBuf.Bytes()), string(stderrBuf.Bytes())
	// log.Debugf("command: %s:\nout:\n%s\nerr:\n%s\n", cmd.String(), outStr, errStr)
	return nil
}
