package deployment

import (
	"context"
	"errors"

	"fmt"
	"strconv"

	"github.com/rancher/rancher/pkg/api/scheme"

	"github.com/rancher/rancher/tests/v2/actions/kubeapi/workloads/deployments"
	"github.com/rancher/rancher/tests/v2/actions/workloads/pods"
	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/extensions/charts"
	"github.com/rancher/shepherd/extensions/kubectl"
	"github.com/rancher/shepherd/extensions/unstructured"
	"github.com/rancher/shepherd/extensions/workloads"
	namegen "github.com/rancher/shepherd/pkg/namegenerator"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	imageName           = "nginx"
	historyHeader       = "REVISION  CHANGE-CAUSE"
	historyHeaderLength = 2
	revisionsIndex      = 1
	revisionNumberIndex = 0
	revisionAnnotation  = "deployment.kubernetes.io/revision"
)

// CreateDeploymentWithConfigmap is a helper to create a deployment with or without a secret/configmap
func CreateDeploymentWithConfigmap(client *rancher.Client, clusterID, namespaceName string, replicaCount int, secretName, configMapName string, useEnvVars, useVolumes bool) (*appv1.Deployment, error) {
	deploymentName := namegen.AppendRandomString("testdeployment")
	containerName := namegen.AppendRandomString("testcontainer")
	pullPolicy := corev1.PullAlways
	replicas := int32(replicaCount)

	var podTemplate corev1.PodTemplateSpec

	if secretName != "" || configMapName != "" {
		podTemplate = pods.NewPodTemplateWithConfig(secretName, configMapName, useEnvVars, useVolumes)
	} else {
		containerTemplate := workloads.NewContainer(
			containerName,
			imageName,
			pullPolicy,
			[]corev1.VolumeMount{},
			[]corev1.EnvFromSource{},
			nil,
			nil,
			nil,
		)
		podTemplate = workloads.NewPodTemplate(
			[]corev1.Container{containerTemplate},
			[]corev1.Volume{},
			[]corev1.LocalObjectReference{},
			nil,
			nil,
		)
	}

	createdDeployment, err := deployments.CreateDeployment(client, clusterID, deploymentName, namespaceName, podTemplate, replicas)
	if err != nil {
		return nil, err
	}

	err = charts.WatchAndWaitDeployments(client, clusterID, namespaceName, metav1.ListOptions{
		FieldSelector: "metadata.name=" + createdDeployment.Name,
	})

	return createdDeployment, err
}

// UpdateDeployment is a helper to update deployments
func UpdateDeployment(client *rancher.Client, clusterID, namespaceName string, deployment *appv1.Deployment) (*appv1.Deployment, error) {
	dynamicClient, err := client.GetDownStreamClusterClient(clusterID)
	if err != nil {
		return nil, err
	}

	deploymentResource := dynamicClient.Resource(deployments.DeploymentGroupVersionResource).Namespace(namespaceName)

	unstructuredResp, err := deploymentResource.Get(context.TODO(), deployment.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	latestDeployment := &appv1.Deployment{}
	err = scheme.Scheme.Convert(unstructuredResp, latestDeployment, unstructuredResp.GroupVersionKind())
	if err != nil {
		return nil, err
	}

	deployment.ResourceVersion = latestDeployment.ResourceVersion

	unstructuredResp, err = deploymentResource.Update(context.TODO(), unstructured.MustToUnstructured(deployment), metav1.UpdateOptions{})
	if err != nil {
		return nil, err
	}

	updatedDeployment := &appv1.Deployment{}
	err = scheme.Scheme.Convert(unstructuredResp, updatedDeployment, unstructuredResp.GroupVersionKind())
	if err != nil {
		return nil, err
	}

	err = charts.WatchAndWaitDeployments(client, clusterID, namespaceName, metav1.ListOptions{
		FieldSelector: "metadata.name=" + updatedDeployment.Name,
	})

	return updatedDeployment, err
}

// RolbackDeployment is a helper to rollback deployments
func RollbackDeployment(client *rancher.Client, clusterID, namespaceName string, deploymentName string, revision int) (string, error) {
	deploymentCmd := fmt.Sprintf("deployment.apps/%s", deploymentName)
	revisionCmd := fmt.Sprintf("--to-revision=%s", strconv.Itoa(revision))
	execCmd := []string{"kubectl", "rollout", "undo", "-n", namespaceName, deploymentCmd, revisionCmd}
	logCmd, err := kubectl.Command(client, nil, clusterID, execCmd, "")
	return logCmd, err
}

// ValidateRolloutHistoryDeployment is a helper to validate rollout history deployment
func ValidateRolloutHistoryDeployment(client *rancher.Client, clusterID, namespaceName string, deploymentName string, expectedRevision string) error {
	dynamicClient, err := client.GetDownStreamClusterClient(clusterID)
	if err != nil {
		return err
	}

	deploymentResource := dynamicClient.Resource(deployments.DeploymentGroupVersionResource).Namespace(namespaceName)

	unstructuredResp, err := deploymentResource.Get(context.TODO(), deploymentName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	latestDeployment := &appv1.Deployment{}
	err = scheme.Scheme.Convert(unstructuredResp, latestDeployment, unstructuredResp.GroupVersionKind())
	if err != nil {
		return err
	}

	if latestDeployment.ObjectMeta.Annotations == nil {
		return errors.New("revision empty")
	}

	revision := latestDeployment.ObjectMeta.Annotations[revisionAnnotation]

	if revision == expectedRevision {
		return nil
	}

	return errors.New("revision not found")
}
