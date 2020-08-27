package dockersquash

import "testing"

func TestSquashImage(t *testing.T) {
	err := DockerSquash("postgres:9.6.17-alpine", "mysecondimage:squashed")
	if err != nil {
		t.Errorf("%+v", err)
	}
}
