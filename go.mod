module github.com/blackducksoftware/kubectl-bd-xray

go 1.15

require (
	cloud.google.com/go v0.65.0 // indirect
	github.com/Azure/go-autorest/autorest v0.11.4 // indirect
	github.com/Azure/go-autorest/autorest/adal v0.9.2 // indirect
	github.com/aquasecurity/fanal v0.0.0-20200820074632-6de62ef86882
	github.com/asaskevich/govalidator v0.0.0-20200819183940-29e1ff8eb0bb // indirect
	github.com/containerd/containerd v1.4.0 // indirect
	github.com/docker/cli v0.0.0-20200130152716-5d0cf8839492 // indirect
	github.com/docker/docker v1.4.2-0.20200203170920-46ec8731fbce
	github.com/go-openapi/errors v0.19.6 // indirect
	github.com/go-openapi/strfmt v0.19.5 // indirect
	github.com/go-resty/resty/v2 v2.3.0
	github.com/google/go-containerregistry v0.1.2
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/googleapis/gnostic v0.5.1 // indirect
	github.com/imdario/mergo v0.3.11 // indirect
	github.com/jedib0t/go-pretty v4.3.0+incompatible
	github.com/mattn/go-runewidth v0.0.9 // indirect
	github.com/mcuadros/go-version v0.0.0-20190830083331-035f6764e8d2
	github.com/mitchellh/mapstructure v1.3.3 // indirect
	github.com/oklog/run v1.1.0
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/pkg/errors v0.9.1
	github.com/sirupsen/logrus v1.6.0
	github.com/spf13/cobra v1.0.0
	go.mongodb.org/mongo-driver v1.4.0 // indirect
	golang.org/x/crypto v0.0.0-20200820211705-5c72a883971a // indirect
	golang.org/x/sys v0.0.0-20200828194041-157a740278f4 // indirect
	golang.org/x/time v0.0.0-20200630173020-3af7569d3a1e // indirect
	google.golang.org/genproto v0.0.0-20200828030656-73b5761be4c5 // indirect
	google.golang.org/grpc v1.31.1 // indirect
	k8s.io/api v0.19.0
	k8s.io/apimachinery v0.19.0
	k8s.io/client-go v11.0.0+incompatible
	k8s.io/klog/v2 v2.3.0 // indirect
	k8s.io/utils v0.0.0-20200821003339-5e75c0163111 // indirect
)

// this repo's dependency is tied to the dependency of helm, mainly for go-autorest
// best place to look is here: https://github.com/helm/helm/blob/v3.3.0/go.mod
replace (
	k8s.io/api => k8s.io/api v0.19.0
	k8s.io/apimachinery => k8s.io/apimachinery v0.19.0
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.19.0
	k8s.io/client-go => k8s.io/client-go v0.19.0
)
