package yaml

import (
	"bufio"
	"os"
	"regexp"
	"strings"
)

// grepping for image in yaml
func GetImageFromYaml(filename string) ([]string, error) {
	list := []string{}

	file, err := os.Open(filename)
	if err != nil {
		return list, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if strings.Contains(scanner.Text(), "image:") {
			imageString := strings.TrimSpace(scanner.Text())
			imageString = strings.TrimPrefix(imageString, "image:")
			imageString = strings.TrimSpace(imageString)
			list = append(list, imageString)
		}
	}

	if err := scanner.Err(); err != nil {
		return list, err
	}

	return list, err
}

func GetImageFromYamlString(templateOutput string) []string {
	list := []string{}

	linesToProcess := strings.Split(templateOutput, "\n")
	repoRegexp := regexp.MustCompile(`image: `)
	for _, line := range linesToProcess {
		repoSubstringSubmatch := repoRegexp.FindStringSubmatch(line)
		if len(repoSubstringSubmatch) > 0 {

			imageString := strings.TrimSpace(line)
			imageString = strings.TrimPrefix(imageString, "image:")
			imageString = strings.TrimSpace(imageString)
			imageString = strings.Trim(imageString, "\"")
			list = append(list, imageString)
		}
	}

	return list
}
