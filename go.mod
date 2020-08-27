module github.com/blackducksoftware/kubectl-bd-xray

go 1.15

require (
	github.com/docker/docker v1.4.2-0.20200203170920-46ec8731fbce
	github.com/ghodss/yaml v1.0.0
	github.com/go-resty/resty/v2 v2.3.0
	github.com/imdario/mergo v0.3.11
	github.com/jedib0t/go-pretty v4.3.0+incompatible
	github.com/oklog/run v1.1.0
	github.com/pkg/errors v0.9.1
	github.com/sirupsen/logrus v1.6.0
	github.com/spf13/cobra v1.0.0
	helm.sh/helm/v3 v3.3.0
	k8s.io/api v0.18.8
	k8s.io/apimachinery v0.18.8
	k8s.io/cli-runtime v0.18.8
	k8s.io/client-go v11.0.0+incompatible
	k8s.io/klog v1.0.0
)

replace (
	// this repo's dependency is tied to the dependency of helm, mainly for go-autorest
	// best place to look is here: https://github.com/helm/helm/blob/v3.3.0/go.mod
	helm.sh/helm/v3 => helm.sh/helm/v3 v3.3.0
	github.com/Azure/go-autorest => github.com/Azure/go-autorest v13.3.2+incompatible
	k8s.io/api => k8s.io/api v0.18.4
	k8s.io/apimachinery => k8s.io/apimachinery v0.18.4
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.18.4
	k8s.io/client-go => k8s.io/client-go v0.18.4
)
