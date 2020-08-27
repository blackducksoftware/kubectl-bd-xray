package dockersquash

import (
	"fmt"
	"github.com/blackducksoftware/kubectl-bd-xray/pkg/util"
	"os/exec"
)


func DockerSquash(imageName, tagName string) error {
	// Ensure docker-squash is installed
	command := "pip install docker-squash"
	cmd := exec.Command("sh", "-c", command)
	_, err := util.RunCommand(cmd)
	if err != nil {
		return err
	}

	// Squash the image
	command = fmt.Sprintf("docker-squash -t %s %s", tagName, imageName)
	cmd = exec.Command("sh", "-c", command)
	_, err = util.RunCommand(cmd)
	if err != nil {
		return err
	}

	return nil
}