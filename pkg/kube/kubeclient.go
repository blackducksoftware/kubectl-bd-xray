package kube

import (
	"context"
	"fmt"
	"os"
	"path"

	v1 "k8s.io/api/core/v1"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"

	appsv1 "k8s.io/api/apps/v1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth" // required for auth, see: https://github.com/kubernetes/client-go/tree/v0.17.3/plugin/pkg/client/auth
	"k8s.io/client-go/tools/clientcmd"
)

type Client struct {
	Clientset *kubernetes.Clientset
}

func PathToKubeConfig() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", errors.Wrapf(err, "unable to get home dir")
	}
	return path.Join(home, ".kube", "config"), nil
}

func NewDefaultClient() (*Client, error) {
	kubeConfigPath, err := PathToKubeConfig()
	if err != nil {
		return nil, err
	}
	return NewClient(kubeConfigPath)
}

func NewClient(kubeConfigPath string) (*Client, error) {
	log.Debugf("instantiating k8s client from config path: '%s'", kubeConfigPath)
	kubeConfig, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	// kubeConfig, err := rest.InClusterConfig()
	if err != nil {
		return nil, errors.Wrapf(err, "unable to build config from flags")
	}
	client, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to instantiate client")
	}
	return &Client{
		Clientset: client,
	}, nil
}

func (kc *Client) ListNamespaces() (*v1.NamespaceList, error) {
	namespaces, err := kc.Clientset.CoreV1().Namespaces().List(metav1.ListOptions{})
	return namespaces, errors.Wrapf(err, "unable to get namesapces")
}

func (kc *Client) GetNamespace(namespace string) (*v1.Namespace, error) {
	ns, err := kc.Clientset.CoreV1().Namespaces().Get(namespace, metav1.GetOptions{})
	return ns, errors.Wrapf(err, "unable to get namespace '%s'", namespace)
}

func (kc *Client) ListPods(ctx context.Context, namespace string, label string, value string) (*v1.PodList, error) {
	selector := fmt.Sprintf("%s=%s", label, value)
	pods, err := kc.Clientset.CoreV1().Pods(namespace).List(metav1.ListOptions{
		LabelSelector: selector,
	})
	return pods, errors.Wrapf(err, "unable to list pods in ns '%s' with label selector '%s", namespace, selector)
}

func (kc *Client) ListDeployments(ctx context.Context, namespace string) (*appsv1.DeploymentList, error) {
	log.Infof("listing deployments in namespace: '%s'; equivalent to 'kubectl get deployments -n %s'", namespace, namespace)
	deploymentList, err := kc.Clientset.AppsV1().Deployments(namespace).List(metav1.ListOptions{})
	return deploymentList, errors.Wrapf(err, "could not get a list of deployments in namespace: '%s'", namespace)
}

func (kc *Client) ListStatefulSets(ctx context.Context, namespace string) (*appsv1.StatefulSetList, error) {
	log.Infof("listing statefulsets in namespace: '%s'; equivalent to 'kubectl get statefulsets -n %s'", namespace, namespace)
	statefulSetList, err := kc.Clientset.AppsV1().StatefulSets(namespace).List(metav1.ListOptions{})
	return statefulSetList, errors.Wrapf(err, "could not get a list of deployments in namespace: '%s'", namespace)
}

func (kc *Client) ListCronJobs(ctx context.Context, namespace string) (*batchv1beta1.CronJobList, error) {
	log.Infof("listing cronjobs in namespace: '%s'; equivalent to 'kubectl get cronjobs -n %s'", namespace, namespace)
	cronJobList, err := kc.Clientset.BatchV1beta1().CronJobs(namespace).List(metav1.ListOptions{})
	return cronJobList, errors.Wrapf(err, "could not get a list of deployments in namespace: '%s'", namespace)
}
