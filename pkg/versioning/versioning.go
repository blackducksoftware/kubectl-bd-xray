package versioning

import (
	"regexp"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/mcuadros/go-version"
)

const (
	validReleaseSemverRegex = "^(v?[0-9]*\\.?[0-9]*\\.?[0-9]*)$"
	validSemverRegex        = "^(v?[0-9]*\\.?[0-9]*\\.?[0-9]*)(-[a-z0-9.]+)?$"
	// Major means a major difference between two versions
	Major = "MAJOR"
	// Minor means a minor difference between two versions
	Minor = "MINOR"
	// Patch means a patch difference between two versions
	Patch = "PATCH"
	// Same means two versions are the same
	Same = "SAME"
	// Unknown means the difference between the two versions is unknown
	Unknown = "UNKNOWN"

	// Notfound means a version could not be found
	Notfound = "NOTFOUND"
	// Failure means something went wrong went finding the version
	Failure = "FAILURE"
	// Nodata indicates there was not a failure but there wasn't any data
	Nodata = "NODATA"
)

var regexRelease *regexp.Regexp
var regex *regexp.Regexp

func init() {
	var err error
	regexRelease, err = regexp.Compile(validReleaseSemverRegex)
	if err != nil {
		log.WithError(err).Fatal("Could not create regexRelease")
	}

	regex, err = regexp.Compile(validSemverRegex)
	if err != nil {
		log.WithError(err).Fatalf("Could not create regex")
	}
}

//FindHighestVersionInList finds the highest version in an list of versions or returns NOTFOUND
func FindHighestVersionInList(versions []string, allowAllReleases bool) string {
	log.WithField("versions", versions).Debug("FindHighestVersionInList")
	latestVersion := "0"

	regexpToUse := regexRelease
	if allowAllReleases {
		regexpToUse = regex
	}

	for _, vers := range versions {
		if !strings.Contains(vers, ".") {
			continue
		}
		if regexpToUse.MatchString(vers) {
			if version.CompareSimple(version.Normalize(vers), version.Normalize(latestVersion)) == 1 {
				latestVersion = vers
			}
		}
	}

	if latestVersion != "0" {
		return latestVersion
	}
	return Notfound
}

// DetermineLifeCycleStatus compares two versions to determin the status of the difference
func DetermineLifeCycleStatus(latestVersion string, currentVersion string) string {
	log.WithField("version", currentVersion).WithField("latestVersion", latestVersion).Debug("Determin status for version")
	latest := strings.Split(version.Normalize(latestVersion), ".")
	curr := strings.Split(version.Normalize(currentVersion), ".")

	if version.Compare(currentVersion, latestVersion, "=") {
		return Same
	}
	if version.Compare(curr[0], latest[0], "<") {
		return Major
	}

	// has minor
	if len(latest) >= 2 && len(curr) >= 2 {
		if version.Compare(curr[1], latest[1], "<") {
			return Minor
		}
	}

	// has patch
	if len(latest) >= 3 && len(curr) >= 3 {
		if version.Compare(curr[2], latest[2], "<") {
			return Patch
		}
	}

	return Unknown
}
