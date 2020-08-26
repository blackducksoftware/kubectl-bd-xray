package detect

import (
	"fmt"

	"github.com/blackducksoftware/kubectl-bd-xray/pkg/util"
)

func FindScanStatusFile(path string) (string, error) {
	_, directory_names, filenames, err := util.GetFilesAndDirectories(path)
	if err != nil {
		return "", err
	}
	for _, filename := range filenames {
		if filename == "status.json" {
			return fmt.Sprintf("%s/status.json", path), nil
		}
	}
	for _, dirname := range directory_names {
		return FindScanStatusFile(fmt.Sprintf("%s/%s", path, dirname))
	}
	return "", err
}
