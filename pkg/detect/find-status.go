package detect

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"

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

type Status struct {
	FormatVersion  string        `json:"formatVersion"`
	DetectVersion  string        `json:"detectVersion"`
	ProjectName    string        `json:"projectName"`
	ProjectVersion string        `json:"projectVersion"`
	Detectors      []interface{} `json:"detectors"`
	Status         []struct {
		Key    string `json:"key"`
		Status string `json:"status"`
	} `json:"status"`
	Issues  []interface{} `json:"issues"`
	Results []struct {
		// TODO: this is what we want
		Location string `json:"location"`
		Message  string `json:"message"`
	} `json:"results"`
	UnrecognizedPaths struct {
	} `json:"unrecognizedPaths"`
	CodeLocations []struct {
		CodeLocationName string `json:"codeLocationName"`
	} `json:"codeLocations"`
}

func ParseStatusJSONFile(path string) (*Status, error) {
	var status Status

	jsonFile, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer jsonFile.Close()

	byteValue, _ := ioutil.ReadAll(jsonFile)
	json.Unmarshal(byteValue, &status)
	return &status, nil
}

func FindLocationFromStatus(status *Status) []string {
	var locations []string
	for _, result := range status.Results {
		locations = append(locations, result.Location)
	}
	return locations
}
