package dockersquash

import "testing"

func TestDownloadDockerSquash(t *testing.T) {
	cli := NewDefaultClient()
	err := cli.DownloadDockerSquash()
	if err != nil {
		t.Errorf("%+v", err)
	}
}