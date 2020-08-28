package helm

import (
	"github.com/blackducksoftware/kubectl-bd-xray/pkg/util"
	"fmt"
)

func TemplateChart(chartURL string) (string, error) {
	cmd := util.GetExecCommandFromString(fmt.Sprintf("helm template temp %s", chartURL))
	template, err := util.RunCommand(cmd)
	if err != nil {
		return template, err
	}
	return template, nil
}