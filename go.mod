module github.com/blackducksoftware/kubectl-bd-xray

go 1.15

require (
	github.com/aquasecurity/fanal v0.0.0-20200820074632-6de62ef86882
	github.com/docker/docker v1.13.1
	github.com/go-openapi/strfmt v0.19.5 // indirect
	github.com/go-resty/resty/v2 v2.3.0
	github.com/google/go-containerregistry v0.1.2
	github.com/jedib0t/go-pretty v4.3.0+incompatible
	github.com/mcuadros/go-version v0.0.0-20190830083331-035f6764e8d2
	github.com/oklog/run v1.1.0
	github.com/pkg/errors v0.9.1
	github.com/sirupsen/logrus v1.6.0
	github.com/spf13/cobra v1.0.0
	k8s.io/api v0.19.0
	k8s.io/apimachinery v0.19.0
	k8s.io/client-go v11.0.0+incompatible
)

replace (
	github.com/docker/docker => github.com/docker/docker v1.4.2-0.20190924003213-a8608b5b67c7
	k8s.io/api => k8s.io/api v0.19.0
	k8s.io/apimachinery => k8s.io/apimachinery v0.19.0
	k8s.io/client-go => k8s.io/client-go v0.19.0
)
