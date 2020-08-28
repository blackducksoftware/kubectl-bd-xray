package remediation

import (
	"sort"
	"sync"

	log "github.com/sirupsen/logrus"

	"github.com/blackducksoftware/kubectl-bd-xray/pkg/registries"
)

// Image holds the Docker image information of the container running in the cluster
type Image struct {
	FullPath string
	URL      string
	Name     string
	Version  string
}

// ContainerInfo contains pod information about the container, its version info, and security
type ContainerInfo struct {
	Container                  Image
	LatestVersion              string
	Fetched                    bool
	VulnerabilitiesNotAccepted int
}

// GetLatestVersionsForImages contains gets the latest version of images
func GetLatestVersionsForImages(containers []Image, registries registries.ImageRegistries) []ContainerInfo {
	var wg sync.WaitGroup
	var containerInfo []ContainerInfo
	queue := make(chan ContainerInfo, 1)
	wg.Add(len(containers))
	log.WithField("lcm", "getLatestVersionsForContainers").Debugf("all containers slice is %+v", containers)
	for _, container := range containers {
		log.WithField("lcm", "getLatestVersionsForContainers").Debugf("current container is %+v", container)
		go func(container Image) {
			version := registries.GetLatestVersionForImage(container.Name, container.URL)
			newContainerInfo := ContainerInfo{
				Container:     container,
				LatestVersion: version,
			}
			queue <- newContainerInfo
		}(container)
	}

	go func() {
		for t := range queue {
			containerInfo = append(containerInfo, t)
			wg.Done()
		}
	}()

	wg.Wait()
	log.WithField("lcm", "getLatestVersionsForContainers").Debugf("containerInfo slice is %+v", containerInfo)

	sort.Slice(containerInfo, func(i, j int) bool {
		return containerInfo[i].Container.Name < containerInfo[j].Container.Name
	})
	return containerInfo
}
