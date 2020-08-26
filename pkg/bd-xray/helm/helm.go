package util

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/ghodss/yaml"
	"github.com/imdario/mergo"
	log "github.com/sirupsen/logrus"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/kube"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/repo"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/rest"
	"k8s.io/klog"
)

var settings = cli.New()

// CreateWithHelm3 uses the helm NewInstall action to create a resource in the cluster
// Modified from https://github.com/openshift/console/blob/cdf6b189b71e488033ecaba7d90258d9f9453478/pkg/helm/actions/install_chart.go
// Helm Actions: https://github.com/helm/helm/tree/9bc7934f350233fa72a11d2d29065aa78ab62792/pkg/action
func CreateWithHelm3(releaseName, namespace, chartURL string, vals map[string]interface{}, kubeConfig string, dryRun bool, extraFiles ...string) error {
	// Check if resouce already exists
	existingRelease, _ := GetWithHelm3(releaseName, namespace, kubeConfig)
	if existingRelease != nil {
		return fmt.Errorf("release '%s' already exists in namespace '%s'", existingRelease.Name, existingRelease.Namespace)
	}

	// Create the new Release
	actionConfig, err := CreateHelmActionConfiguration(kubeConfig, "", namespace)
	if err != nil {
		return fmt.Errorf("failed to create the release config due to %s", err)
	}

	chart, err := LoadChart(chartURL, actionConfig)
	if err != nil {
		return fmt.Errorf("failed to load application resources at '%s' due to %s", chartURL, err)
	}
	validInstallableChart, err := isChartInstallable(chart)
	if !validInstallableChart {
		return fmt.Errorf("release at '%s' is not installable: %s", chartURL, err)
	}
	if chart.Metadata.Deprecated {
		log.Warnf("the release at '%s' is deprecated", chartURL)
	}

	client := action.NewInstall(actionConfig)
	if client.Version == "" && client.Devel {
		client.Version = ">0.0.0-0"
	}
	client.ReleaseName = releaseName
	client.Namespace = namespace
	client.DryRun = dryRun

	fileValues := map[string]interface{}{}
	if err := mergeValuesWithExtraFilesFromChart(chart, fileValues, extraFiles); err != nil {
		return fmt.Errorf("failed to merge extra configuration files during create due to %s", err)
	}
	vals = MergeMaps(fileValues, vals)

	_, err = client.Run(chart, vals) // deploy the chart into the namespace from the actionConfig
	if err != nil {
		return fmt.Errorf("failed to run install due to %s", err)
	}
	return nil
}

// UpdateWithHelm3 uses the helm NewUpgrade action to update a resource in the cluster
func UpdateWithHelm3(releaseName, namespace, chartURL string, vals map[string]interface{}, kubeConfig string, extraFiles ...string) error {
	actionConfig, err := CreateHelmActionConfiguration(kubeConfig, "", namespace)
	if err != nil {
		return err
	}
	if releaseExists := ReleaseExists(releaseName, namespace, kubeConfig); !releaseExists {
		return fmt.Errorf("release '%s' does not exist", releaseName)
	}

	chart, err := LoadChart(chartURL, actionConfig)
	if err != nil {
		return fmt.Errorf("failed to load release at '%s' for updating: %s", chartURL, err)
	}
	validInstallableChart, err := isChartInstallable(chart)
	if !validInstallableChart {
		return fmt.Errorf("release at '%s' is not installable: %s", chartURL, err)
	}

	client := action.NewUpgrade(actionConfig)
	if client.Version == "" && client.Devel {
		client.Version = ">0.0.0-0"
	}
	client.Namespace = namespace

	if err := mergeValuesWithExtraFilesFromChart(chart, vals, extraFiles); err != nil {
		return fmt.Errorf("failed to merge extra configuration files during update due to %s", err)
	}

	client.ResetValues = true                     // rememeber the values that have been set previously
	_, err = client.Run(releaseName, chart, vals) // updates the release in the namespace from the actionConfig
	if err != nil {
		return fmt.Errorf("failed to run upgrade: %s", err)
	}
	return nil
}

// TemplateWithHelm3 prints the kube manifest files for a resource
func TemplateWithHelm3(releaseName, namespace, chartURL string, vals map[string]interface{}, extraFiles ...string) error {
	actionConfig, err := CreateHelmActionConfiguration("", "", namespace)
	if err != nil {
		return err
	}
	chart, err := LoadChart(chartURL, actionConfig)
	if err != nil {
		return err
	}
	validInstallableChart, err := isChartInstallable(chart)
	if !validInstallableChart {
		return err
	}

	fileValues := map[string]interface{}{}
	if err := mergeValuesWithExtraFilesFromChart(chart, fileValues, extraFiles); err != nil {
		return fmt.Errorf("failed to merge extra configuration files during template due to %s", err)
	}
	vals = MergeMaps(fileValues, vals)

	templateOutput, err := RenderManifests(releaseName, namespace, chart, vals, actionConfig)
	if err != nil {
		return fmt.Errorf("failed to render kube manifest files due to %s", err)
	}
	fmt.Printf("%+v\n", templateOutput)
	return nil
}

// RenderManifests converts a helm chart to a string of the kube manifest files
// Modified from https://github.com/openshift/console/blob/cdf6b189b71e488033ecaba7d90258d9f9453478/pkg/helm/actions/template_test.go
func RenderManifests(releaseName, namespace string, chart *chart.Chart, vals map[string]interface{}, actionConfig *action.Configuration) (string, error) {
	validate := false
	includeCrds := true
	emptyResponse := ""

	client := action.NewInstall(actionConfig)
	if client.Version == "" && client.Devel {
		client.Version = ">0.0.0-0"
	}
	client.ReleaseName = releaseName
	client.Namespace = namespace
	client.DryRun = true
	client.Replace = true // Skip the releaseName check
	client.ClientOnly = !validate
	client.IncludeCRDs = includeCrds

	rel, err := client.Run(chart, vals)
	if err != nil {
		return emptyResponse, err
	}

	var manifests bytes.Buffer
	var output bytes.Buffer

	fmt.Fprintln(&manifests, strings.TrimSpace(rel.Manifest))

	if !client.DisableHooks {
		for _, m := range rel.Hooks {
			fmt.Fprintf(&manifests, "---\n# Source: %s\n%s\n", m.Path, m.Manifest)
		}
	}
	fmt.Fprintf(&output, "%s", manifests.String())
	return output.String(), nil
}

// DeleteWithHelm3 uses the helm NewUninstall action to delete a resource from the cluster
func DeleteWithHelm3(releaseName, namespace, kubeConfig string) error {
	actionConfig, err := CreateHelmActionConfiguration(kubeConfig, "", namespace)
	if err != nil {
		return err
	}
	if releaseExists := ReleaseExists(releaseName, namespace, kubeConfig); !releaseExists {
		return fmt.Errorf("release '%s' does not exist", releaseName)
	}
	client := action.NewUninstall(actionConfig)
	_, err = client.Run(releaseName) // deletes the releaseName from the namespace in the actionConfig
	if err != nil {
		return fmt.Errorf("failed to run uninstall due to %s", err)
	}
	return nil
}

// GetWithHelm3 uses the helm NewGet action to return a Release with information about
// a resource from the cluster
func GetWithHelm3(releaseName, namespace, kubeConfig string) (*release.Release, error) {
	actionConfig, err := CreateHelmActionConfiguration(kubeConfig, "", namespace)
	if err != nil {
		return nil, err
	}
	// Check if the Release Exists
	aList := action.NewList(actionConfig) // NewGet provides bad error message if release doesn't exist
	charts, err := aList.Run()
	if err != nil {
		return nil, fmt.Errorf("failed to run get due to %s", err)
	}
	for _, release := range charts {
		if release.Name == releaseName && release.Namespace == namespace {
			return release, nil
		}
	}
	return nil, fmt.Errorf("unable to find release '%s' in namespace '%s'", releaseName, namespace)
}

// ConvertFilesFromChartToMap checks whether an file exist in the chart. If exist, it will convert the file to map and return the map
func ConvertFilesFromChartToMap(namespace, kubeConfig, chartURL, fileName string) (map[string]interface{}, error) {
	actionConfig, err := CreateHelmActionConfiguration(kubeConfig, "", namespace)
	if err != nil {
		return nil, err
	}

	chart, err := LoadChart(chartURL, actionConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to load release at '%s' for getting size: %s", chartURL, err)
	}
	validInstallableChart, err := isChartInstallable(chart)
	if !validInstallableChart {
		return nil, err
	}

	fileValues := map[string]interface{}{}
	if err := mergeValuesWithExtraFilesFromChart(chart, fileValues, []string{fileName}); err != nil {
		return nil, fmt.Errorf("failed to get '%s' file from an app resources due to %s", fileName, err)
	}

	return fileValues, nil
}

// CreateHelmActionConfiguration creates an action.Configuration that points to the specified cluster and namespace
func CreateHelmActionConfiguration(kubeConfig, kubeContext, namespace string) (*action.Configuration, error) {
	// TODO: look into using GetActionConfigurations()
	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(kube.GetConfig(kubeConfig, kubeContext, namespace), namespace, "secret", func(format string, v ...interface{}) {}); err != nil {
		return nil, err
	}
	return actionConfig, nil
}

type configFlagsWithTransport struct {
	*genericclioptions.ConfigFlags
	Transport *http.RoundTripper
}

// GetActionConfigurations creates an action.Configuration that points to the specified cluster and namespace
// TODO - this function specifies more values than CreateHelmActionConfiguration(), consider using this
func GetActionConfigurations(host, namespace, token string, transport *http.RoundTripper) *action.Configuration {

	confFlags := &configFlagsWithTransport{
		ConfigFlags: &genericclioptions.ConfigFlags{
			APIServer:   &host,
			BearerToken: &token,
			Namespace:   &namespace,
		},
		Transport: transport,
	}
	inClusterCfg, err := rest.InClusterConfig()

	if err != nil {
		fmt.Print("Running outside cluster, CAFile is unset")
	} else {
		confFlags.CAFile = &inClusterCfg.CAFile
	}
	conf := new(action.Configuration)
	conf.Init(confFlags, namespace, "secrets", klog.Infof)

	return conf
}

// LoadChart returns a chart from the specified chartURL
// Modified from https://github.com/openshift/console/blob/master/pkg/helm/actions/template_test.go
func LoadChart(chartURL string, actionConfig *action.Configuration) (*chart.Chart, error) {
	client := action.NewInstall(actionConfig)

	// Get full path - checks local machine and chart repository
	chartFullPath, err := client.ChartPathOptions.LocateChart(chartURL, settings)
	if err != nil {
		return nil, fmt.Errorf("failed to locate resources at '%s' due to %s", chartURL, err)
	}

	chart, err := loader.Load(chartFullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load resources from '%s' due to %s", chartFullPath, err)
	}
	return chart, nil
}

// ParseChartVersion ...
func ParseChartVersion(chartURL string) string {
	chartPackageSplit := ParsePackageName(chartURL)
	chartVersion := chartPackageSplit[1]
	if chartPackageSplit[2] != "" {
		chartVersion = fmt.Sprintf("%s-%s", chartVersion, chartPackageSplit[2])
	}
	return chartVersion
}

// GetLatestChartVersionForAppVersion ...
func GetLatestChartVersionForAppVersion(chartURLs []string, appName, version string) (string, error) {
	url, err := GetLatestChartURLForAppVersion(chartURLs, appName, version)
	if err != nil {
		return "", fmt.Errorf("%s", err)
	}
	return ParseChartVersion(url), nil
}

// GetLatestChartURLForAppVersion ...
func GetLatestChartURLForAppVersion(chartURLs []string, appName, version string) (string, error) {
	latestChartNum := 0
	latestURL := ""
	for _, url := range chartURLs {
		packageNameSlice := ParsePackageName(url)
		pkgName := packageNameSlice[0]
		pkgVersion := packageNameSlice[1]
		chrtNum := packageNameSlice[2]
		if chrtNum == "" {
			chrtNum = "0"
		}
		if pkgName == appName {
			if CompareVersions(pkgVersion, version) == 0 {
				chrtNumInt, _ := strconv.Atoi(chrtNum)
				if chrtNumInt >= latestChartNum {
					latestURL = url
					latestChartNum = chrtNumInt
				}
			}
		}
	}
	return latestURL, nil
}

// GetLatestChartURLForApp ...
func GetLatestChartURLForApp(chartURLs []string, appName string) (string, error) {
	latestVersion := "0.0.0"
	latestChartNum := 0
	latestURL := ""
	for _, url := range chartURLs {
		packageNameSlice := ParsePackageName(url)
		pkgName := packageNameSlice[0]
		pkgVersion := packageNameSlice[1]
		chrtNum := packageNameSlice[2]
		if chrtNum == "" {
			chrtNum = "0"
		}
		if pkgName == appName {
			if CompareVersions(pkgVersion, latestVersion) >= 0 {
				if CompareVersions(pkgVersion, latestVersion) > 0 {
					latestChartNum = 0 // reset latestChartNum if a greater version is found
				}
				latestVersion = pkgVersion
				chrtNumInt, _ := strconv.Atoi(chrtNum)
				if chrtNumInt >= latestChartNum {
					latestURL = url
					latestChartNum = chrtNumInt
				}
			}
		}
	}
	return latestURL, nil
}

// ParsePackageName returns {app-name, app-version, chart-num}
// synopsys-alert-5.3.1-12 -> [synopsys-alert-5.3.1-12 synopsys-alert 5.3.1 -12 12]
// blackduck-2020.4.2 -> [blackduck-2020.4.2 blackduck 2020.4.2  ]
func ParsePackageName(chartURL string) []string {
	packageNameRegexp := regexp.MustCompile(`([a-z\-]+)-([0-9\.]*[0-9]+)(-([0-9]+))?`)
	packageSubstringSubmatch := packageNameRegexp.FindStringSubmatch(chartURL)
	parsedOutput := []string{"", "", ""}
	if len(packageSubstringSubmatch) > 2 {
		parsedOutput[0] = packageSubstringSubmatch[1]
		parsedOutput[1] = packageSubstringSubmatch[2]
	}
	if len(packageSubstringSubmatch) > 4 {
		parsedOutput[2] = packageSubstringSubmatch[4]
	}

	return parsedOutput
}

// GetChartURLs ...
// if appName == "" then it returns all chart URLs (as seen in Chart.yaml)
// (NOTE: if the package name does not match name in Chart.yaml exactly then it will not be returned)
func GetChartURLs(repoURL, appName string) ([]string, error) {
	chartURLs := []string{}

	actionConfig, err := CreateHelmActionConfiguration("", "", "")
	if err != nil {
		return chartURLs, err
	}

	indexFile, err := GetIndexFile(repoURL, actionConfig)
	if err != nil {
		return chartURLs, err
	}

	indexEntries := indexFile.Entries
	if appName != "" { // if set, only entries for appName will be in indexEntries
		indexEntries = map[string]repo.ChartVersions{
			appName: indexFile.Entries[appName],
		}
	}

	for _, chartVersions := range indexEntries {
		for _, chartVersion := range chartVersions {
			for _, url := range chartVersion.URLs {
				chartURLs = append(chartURLs, url)
			}
		}
	}

	return chartURLs, nil
}

// GetIndexFile ...
func GetIndexFile(repoURL string, actionConfig *action.Configuration) (*repo.IndexFile, error) {
	client := action.NewInstall(actionConfig)
	if client.Version == "" && client.Devel {
		client.Version = ">0.0.0-0"
	}

	username := client.ChartPathOptions.Username
	password := client.ChartPathOptions.Password
	// chartName := chartURL
	// chartVersion :=  strings.TrimSpace(client.ChartPathOptions.Version)
	certFile := client.ChartPathOptions.CertFile
	keyFile := client.ChartPathOptions.KeyFile
	caFile := client.ChartPathOptions.CaFile

	getters := getter.All(settings)

	// Download and write the index file to a temporary location
	buf := make([]byte, 20)
	rand.Read(buf)
	name := strings.ReplaceAll(base64.StdEncoding.EncodeToString(buf), "/", "-")

	c := repo.Entry{
		URL:      repoURL,
		Username: username,
		Password: password,
		CertFile: certFile,
		KeyFile:  keyFile,
		CAFile:   caFile,
		Name:     name,
	}
	r, err := repo.NewChartRepository(&c, getters)
	if err != nil {
		return nil, err
	}
	idx, err := r.DownloadIndexFile()
	if err != nil {
		return nil, fmt.Errorf("looks like %q is not a valid chart repository or cannot be reached: %+v", repoURL, err)
	}

	// Read the index file for the repository to get chart information and return chart URL
	repoIndex, err := repo.LoadIndexFile(idx)
	if err != nil {
		return nil, err
	}

	return repoIndex, nil
}

// isChartInstallable validates if a chart can be installed
//
// Only the "application" chart type is installable
func isChartInstallable(ch *chart.Chart) (bool, error) {
	switch ch.Metadata.Type {
	case "", "application":
		return true, nil
	}
	return false, fmt.Errorf("resources at '%s' are not installable", ch.Metadata.Type)
}

// ReleaseExists verifies that a resources is deployed in the cluster
func ReleaseExists(releaseName, namespace, kubeConfig string) bool {
	actionConfig, err := CreateHelmActionConfiguration(kubeConfig, "", namespace)
	if err != nil {
		return false
	}
	client := action.NewGet(actionConfig)
	release, err := client.Run(releaseName) // lists the releases in the namespace from the actionConfig
	if err != nil || release == nil {
		return false
	}
	return true
}

// mergeValuesWithExtraFilesFromChart merges all extra files from chart with the values map
func mergeValuesWithExtraFilesFromChart(ch *chart.Chart, vals map[string]interface{}, extraFiles []string) error {
	for _, fileName := range extraFiles {
		found := false
		for _, chartFile := range ch.Files {
			if fileName == chartFile.Name {
				found = true
				patch := make(map[string]interface{})
				if err := yaml.Unmarshal(chartFile.Data, &patch); err != nil {
					return err
				}
				if err := mergo.Merge(&vals, patch, mergo.WithOverride); err != nil {
					return err
				}
				break
			}
		}
		if !found {
			return fmt.Errorf("couldn't find file '%s' in release resources", fileName)
		}
	}
	return nil
}

// SetHelmValueInMap adds the finalValue into the valueMapPointer at the location specified
// by the keyList
// valueMapPointer - a map for helm values (maps are pointers in Golang)
//  - it is used to track the current map being updated
// keyList - an ordered list of keys that lead to the location in the valueMapPointer to place the finalValue
// finalValue - the value to set in the map
func SetHelmValueInMap(valueMapPointer map[string]interface{}, keyList []string, finalValue interface{}) {
	for i, currKey := range keyList {
		if i == (len(keyList) - 1) { // at the last key -> set the value
			valueMapPointer[currKey] = finalValue
			return
		}
		if nextMap, _ := valueMapPointer[currKey]; nextMap != nil { // key is in map -> go to next map
			valueMapPointer = nextMap.(map[string]interface{})
		} else { // key is not in the map -> add the key and next key; go to next map
			nextKey := keyList[i+1]
			valueMapPointer[currKey] = map[string]interface{}{nextKey: nil}
			nextMap := valueMapPointer[currKey].(map[string]interface{})
			valueMapPointer = nextMap
		}
	}
}

// GetHelmValueFromMap returns an interface{} if the value exists in the map
func GetHelmValueFromMap(valueMapPointer map[string]interface{}, keyList []string) interface{} {
	for i, currKey := range keyList {
		currVal, ok := valueMapPointer[currKey]
		if !ok { // value doesn't exist - invalid path
			return nil
		}
		if i == (len(keyList) - 1) { // at the last key -> return the value
			return currVal
		}
		valueMapPointer, ok = currVal.(map[string]interface{})
		if !ok { // got a finalValue instead of a map - invalid path
			return nil
		}
	}
	return nil
}

// GetValueFromRelease merges the default Chart Values with the user's set values
// to find the value that is current set in the Release
func GetValueFromRelease(release *release.Release, keyList []string) interface{} {
	chartValues := release.Chart.Values
	userConfig := release.Config
	releaseValues := MergeMaps(chartValues, userConfig)
	return GetHelmValueFromMap(releaseValues, keyList)
}

// DeepCopyHelmValuesMap copies the src map to the dest map. Values in new map have different pointers/references
func DeepCopyHelmValuesMap(src map[string]interface{}, dest map[string]interface{}) error {
	jsonStr, err := json.Marshal(src)
	if err != nil {
		return err
	}
	err = json.Unmarshal(jsonStr, &dest)
	if err != nil {
		return err
	}
	return nil
}

// GetReleaseValues merges the default Chart Values with the user's set values
// and retuns that set of values
func GetReleaseValues(release *release.Release) map[string]interface{} {
	chartValues := release.Chart.Values
	userConfig := release.Config
	return MergeMaps(chartValues, userConfig)
}

// MergeMaps Copied from https://github.com/helm/helm/blob/9b42702a4bced339ff424a78ad68dd6be6e1a80a/pkg/cli/values/options.go#L88
func MergeMaps(a, b map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(a))
	for k, v := range a {
		out[k] = v
	}
	for k, v := range b {
		if v, ok := v.(map[string]interface{}); ok {
			if bv, ok := out[k]; ok {
				if bv, ok := bv.(map[string]interface{}); ok {
					out[k] = MergeMaps(bv, v)
					continue
				}
			}
		}
		out[k] = v
	}
	return out
}

// GetDeploymentResources reads the deployment resource file path and sets the Helm resource maps
// func GetDeploymentResources(deploymentResourceFilePath string, valueMapPointer map[string]interface{}, heapMaxMemoryName string) {
// 	data, err := ReadFileData(deploymentResourceFilePath)
// 	if err != nil {
// 		log.Fatalf("failed to read deployment resources file: %s", err)
// 	}
// 	deploymentResources := make(map[string]api.DeploymentResource, 0)
// 	err = json.Unmarshal([]byte(data), &deploymentResources)
// 	if err != nil {
// 		log.Fatalf("failed to unmarshal deployment resources structs: %s", err)
// 	}

// 	for key, value := range deploymentResources {
// 		if value.Replicas != nil {
// 			SetHelmValueInMap(valueMapPointer, []string{key, "replicas"}, *value.Replicas)
// 		}
// 		if value.HeapMaxMemory != nil {
// 			setStringPtrInHelmValueInMap(valueMapPointer, []string{key, heapMaxMemoryName}, value.HeapMaxMemory)
// 		}
// 		setStringPtrInHelmValueInMap(valueMapPointer, []string{key, "resources", "limits", "cpu"}, value.Resources.Limits.CPU)
// 		setStringPtrInHelmValueInMap(valueMapPointer, []string{key, "resources", "limits", "memory"}, value.Resources.Limits.Memory)
// 		setStringPtrInHelmValueInMap(valueMapPointer, []string{key, "resources", "requests", "cpu"}, value.Resources.Requests.CPU)
// 		setStringPtrInHelmValueInMap(valueMapPointer, []string{key, "resources", "requests", "memory"}, value.Resources.Requests.Memory)
// 	}
// }

func setStringPtrInHelmValueInMap(valueMapPointer map[string]interface{}, keyList []string, value *string) {
	if value != nil {
		SetHelmValueInMap(valueMapPointer, keyList, *value)
	}
}
