package yaml

import (
	"os"
	"bufio"
	"strings"
)

// grepping for image in yaml
func getImageFromYaml(filename string) ([]string, error) {
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