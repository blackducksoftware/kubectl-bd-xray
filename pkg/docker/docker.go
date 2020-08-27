package docker

import (
	"context"
	"fmt"

	"github.com/aquasecurity/fanal/image/daemon"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"

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

func (cli *DockerCLIClient) ListImages(ctx context.Context, reference string) ([]types.ImageSummary, error) {
	// supported filters: https://docs.docker.com/engine/reference/commandline/images/#filtering
	var filterArgs filters.Args
	if reference != "" {
		filterArgs = filters.NewArgs(filters.Arg("reference", reference))
		// fmt.Sprintf("%s:%s", args.Repository, args.Tag)))
	}
	log.Tracef("filter arguments for reference %s: %+v", reference, filterArgs)
	images, err := cli.DockerClient.ImageList(ctx, types.ImageListOptions{
		All:     false,
		Filters: filterArgs,
	})

	return images, errors.Wrapf(err, "unable to list images")
}

// TODO: not working
func (cli *DockerCLIClient) GetDockerImage(ctx context.Context, ref name.Reference) error {
	imageInspect, _, err := cli.DockerClient.ImageInspectWithRaw(ctx, ref.Name())
	log.Infof("imageInspect looks like: %+v", imageInspect)
	log.Infof("\n\n id looks: %s\n\n", imageInspect.ID)
	return err
}

func (cli *DockerCLIClient) GetImageSha(image string) (string, error) {
	var shaOfImage string

	ref, err := name.ParseReference(image)
	if err != nil {
		return "", err
	}
	log.Tracef("reference: %s", ref)
	img, cleanup, _ := daemon.Image(ref)
	defer cleanup()
	log.Tracef("image: %s", img)

	// imageSummary, _ := dockerCLIClient.ListImages(context.TODO(), ref.Name())
	// for _, x := range imageSummary {
	// 	log.Infof("ID: %s", x.ID)
	// }

	// dockerCLIClient.GetDockerImage(context.TODO(), ref)

	cfgName, err := img.ConfigName()
	if err != nil {
		return "", err
	}
	shaOfImage = cfgName.Hex
	log.Tracef("image digest: %s", shaOfImage)
	return shaOfImage, nil
}

// SaveDockerImage creates a tar as an un-squashed image
func (cli *DockerCLIClient) SaveDockerImage(image, filePath string) error {
	// TODO: use golang client instead of docker
	// cli.DockerClient.ImageSave()
	cmd := util.GetExecCommandFromString(fmt.Sprintf("docker save -o %s %s", filePath, image))
	var err error
	_, err = util.RunCommand(cmd)
	return err
}

func (cli *DockerCLIClient) SaveDockerImageAsTarGz(image, filePath string) error {
	cmd := util.GetExecCommandFromString(fmt.Sprintf("sh -c docker save %s | gzip > %s.tar.gz", image, filePath))
	var err error
	_, err = util.RunCommand(cmd)
	return err
}

func (cli *DockerCLIClient) PullDockerImage(image string) error {
	cmd := util.GetExecCommandFromString(fmt.Sprintf("docker pull %s", image))
	var err error
	_, err = util.RunCommand(cmd)
	return err
}

// func SquashDockerImage() {

// }

// func (cli *Client) GetImageSha(image string) {
// 	info, _, err = cli.ImageInspectWithRaw(image, false)
// }
