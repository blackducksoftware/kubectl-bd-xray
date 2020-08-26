module github.com/blackducksoftware/kubectl-bd-xray

go 1.15

require (
	github.com/pkg/errors v0.9.1
	github.com/sirupsen/logrus v1.6.0
	github.com/spf13/cobra v1.0.0
	k8s.io/cli-runtime v0.18.8
	k8s.io/client-go v11.0.0+incompatible
)

replace k8s.io/client-go v11.0.0+incompatible => k8s.io/client-go v0.18.8
