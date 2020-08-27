package docker

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/pkg/errors"

	"github.com/blackducksoftware/kubectl-bd-xray/pkg/util"
)

type DockerCLIClient struct {
	DockerClient *client.Client
}

func NewCliClient() (*DockerCLIClient, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to instantiate docker cli")
	}
	return &DockerCLIClient{DockerClient: cli}, nil
}

func (cli *DockerCLIClient) ListImages(reference string) ([]types.ImageSummary, error) {
	// supported filters: https://docs.docker.com/engine/reference/commandline/images/#filtering
	var filterArgs filters.Args
	if reference != "" {
		filterArgs = filters.NewArgs(filters.Arg("reference", reference))
		// fmt.Sprintf("%s:%s", args.Repository, args.Tag)))
	}
	images, err := cli.DockerClient.ImageList(context.TODO(), types.ImageListOptions{
		All:     false,
		Filters: filterArgs,
	})
	return images, errors.Wrapf(err, "unable to list images")
}

func (cli *DockerCLIClient) SaveDockerImage(image, filePath string) error {
	// TODO: use golang client instead of docker
	// cli.DockerClient.ImageSave()
	cmd := util.GetExecCommandFromString(fmt.Sprintf("docker save -o %s %s", filePath, image))
	var err error
	_, err = util.RunCommand(cmd)
	return err
}

// func SquashDockerImage() {

// }

// func (cli *Client) GetImageSha(image string) {
// 	info, _, err = cli.ImageInspectWithRaw(image, false)
// }
