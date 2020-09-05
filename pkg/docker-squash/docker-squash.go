package dockersquash

import (
	"fmt"
	"os"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"

	"github.com/blackducksoftware/kubectl-bd-xray/pkg/utils"
)

const DockerSquashPath = "docker-squash"

func DockerSquash(imageName, outputFilePath string) error {
	// Ensure docker-squash is installed
	err := DownloadDockerSquashIfNotInstalled()
	if err != nil {
		return err
	}

	// Squash the image
	cmd := utils.GetExecCommandFromString(fmt.Sprintf("docker-squash %s --output-path %s", imageName, outputFilePath))
	_, err = utils.RunCommand(cmd)
	return err
}

// TODO: remove dependency on python
func PipInstallDockerSquash() error {
	cmd := utils.GetExecCommandFromString(fmt.Sprintf("pip install docker-squash"))
	var err error
	_, err = utils.RunCommand(cmd)
	return err
}

func DownloadDockerSquashIfNotInstalled() error {
	if _, err := os.Stat(DockerSquashPath); err == nil {
		log.Debugf("detect found at %s, not downloading again, running sync recommended", DockerSquashPath)
		// TODO: sync to latest version if possible
		return nil
	} else if os.IsNotExist(err) {
		log.Debugf("%s not found at %s, downloading ...", DockerSquashPath, DockerSquashPath)
		// if docker-squash not found at path, then download a fresh copy
		return PipInstallDockerSquash()
	} else {
		return errors.Wrapf(err, "unable to check if file %s exists", DockerSquashPath)
	}
}
