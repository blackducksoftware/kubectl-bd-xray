package helm

import (
	"fmt"
	"github.com/blackducksoftware/kubectl-bd-xray/pkg/utils"
)

func TemplateChart(chartURL string) (string, error) {
	cmd := utils.GetExecCommandFromString(fmt.Sprintf("helm template temp %s", chartURL))
	template, err := utils.RunCommand(cmd)
	if err != nil {
		return template, err
	}
	return template, nil
}
