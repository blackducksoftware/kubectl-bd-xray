package dockersquash

import (
	"github.com/blackducksoftware/kubectl-bd-xray/pkg/docker"
	"github.com/blackducksoftware/kubectl-bd-xray/pkg/util"
	"github.com/go-resty/resty/v2"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"os"
	"time"
)

const (
	DefaultDockerSquashPath = "./docker-squash"
	DefaultDockerSquashURL              = "https://github.com/jwilder/docker-squash/releases/download/v0.2.0/docker-squash-darwin-amd64-v0.2.0.tar.gz"
)

type Client struct {
	DockerSquashPath      string
	DockerSquashURL       string
	RestyClient     *resty.Client
	DockerCLIClient *docker.DockerCLIClient
}

func NewDefaultClient() *Client {
	return NewClient(
		DefaultDockerSquashPath,
		DefaultDockerSquashURL)
}

func NewClient(dockerSquashPath, dockerSquashURL string) *Client {
	restyClient := resty.New().
		// for all requests, you can use relative path; resty doesn't care whether relative path starts with /
		SetHostURL(dockerSquashURL).
		// exponential backoff: https://github.com/go-resty/resty#retries
		SetRetryCount(3).
		// set this to true if you want to get more info, including response metrics
		SetDebug(false).
		SetTimeout(180 * time.Second)

	dockerCLIClient, err := docker.NewCliClient()
	util.DoOrDie(err)

	return &Client{
		DockerSquashPath: dockerSquashPath,
		DockerSquashURL:       dockerSquashURL,
		RestyClient:     restyClient,
		DockerCLIClient: dockerCLIClient,
	}
}

func (c *Client) DownloadDockerSquash() error {
	log.Infof("Downloading Docker-Squash")
	request := c.RestyClient.R().
		// see: https://github.com/go-resty/resty#save-http-response-into-file
		SetOutput(c.DockerSquashPath)
	resp, err := request.Get(c.DockerSquashURL)
	if err != nil {
		return errors.Wrapf(err, "issue GET request to %s", request.URL)
	}
	respBody, statusCode := resp.String(), resp.StatusCode()
	if !resp.IsSuccess() {
		return errors.Errorf("bad status code to path %s: %d, response %s", c.DockerSquashURL, statusCode, respBody)
	}
	log.Infof("Running chmod on Docker-Squash")
	_, err = util.ChmodX(c.DockerSquashPath)
	return err
}

func (c *Client) DownloadDockerSquashIfNotExists() error {
	if _, err := os.Stat(c.DockerSquashPath); err == nil {
		log.Debugf("detect found at %s, not downloading again, running sync recommended", c.DockerSquashPath)
		// TODO: sync to latest version of the specified Black Duck server, if possible
		return nil
	} else if os.IsNotExist(err) {
		log.Debugf("detect not found at %s, downloading ...", c.DockerSquashPath)
		// if detect not found at path, then download a fresh copy
		return c.DownloadDockerSquash()
	} else {
		return errors.Wrapf(err, "unable to check if file %s exists", c.DockerSquashPath)
	}
}

func (c *Client) GetImageSha() (string, error) {
	// Given a name and tag -> get the sha

	// `docker inspect`

	return "", nil
}