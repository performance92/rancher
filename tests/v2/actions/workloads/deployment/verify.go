package deployment

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rancher/rancher/tests/v2/actions/workloads/pods"
	"github.com/rancher/shepherd/clients/rancher"
	steveV1 "github.com/rancher/shepherd/clients/rancher/v1"
	"github.com/rancher/shepherd/extensions/charts"
	"github.com/rancher/shepherd/extensions/defaults"
	"github.com/rancher/shepherd/pkg/wrangler"
	log "github.com/sirupsen/logrus"
	appv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kwait "k8s.io/apimachinery/pkg/util/wait"
)

const (
	revisionAnnotation = "deployment.kubernetes.io/revision"
	podSteveType       = "pod"
)

// VerifyDeployment waits for a deployment to be ready in the downstream cluster
func VerifyDeployment(steveClient *steveV1.Client, deployment *steveV1.SteveAPIObject) error {
	err := kwait.PollUntilContextTimeout(context.TODO(), 500*time.Millisecond, defaults.FiveMinuteTimeout, true, func(ctx context.Context) (done bool, err error) {
		deploymentResp, err := steveClient.SteveType(DeploymentSteveType).ByID(deployment.Namespace + "/" + deployment.Name)
		if err != nil {
			return false, nil
		}

		deployment := &appv1.Deployment{}
		err = steveV1.ConvertToK8sType(deploymentResp.JSONResp, deployment)
		if err != nil {

			return false, nil
		}

		if *deployment.Spec.Replicas == deployment.Status.AvailableReplicas {
			return true, nil
		}

		return false, nil
	})

	return err
}

func VerifyDeploymentUpgrade(client *rancher.Client, clusterName string, namespaceName string, appv1Deployment *appv1.Deployment, expectedRevision string, image string, expectedReplicas int) error {
	log.Info("Waiting deployment comes up active")
	err := charts.WatchAndWaitDeployments(client, clusterName, namespaceName, metav1.ListOptions{
		FieldSelector: "metadata.name=" + appv1Deployment.Name,
	})
	if err != nil {
		return err
	}

	log.Info("Waiting for all pods to be running")
	err = pods.WatchAndWaitPodContainerRunning(client, clusterName, namespaceName, appv1Deployment)
	if err != nil {
		return err
	}

	log.Infof("Verifying rollout history by revision %s", expectedRevision)
	err = VerifyDeploymentRolloutHistory(client, clusterName, namespaceName, appv1Deployment.Name, expectedRevision)
	if err != nil {
		return err
	}

	log.Infof("Counting all pods running by image %s", image)
	countPods, err := pods.CountPodContainerRunningByImage(client, clusterName, namespaceName, image)
	if err != nil {
		return err
	}

	if expectedReplicas != countPods {
		err_msg := fmt.Sprintf("expected replica count: %d does not equal pod count: %d", expectedReplicas, countPods)
		return errors.New(err_msg)
	}

	return err
}

func VerifyDeploymentScale(client *rancher.Client, clusterName string, namespaceName string, scaleDeployment *appv1.Deployment, image string, expectedReplicas int) error {
	log.Info("Waiting deployment comes up active")
	err := charts.WatchAndWaitDeployments(client, clusterName, namespaceName, metav1.ListOptions{
		FieldSelector: "metadata.name=" + scaleDeployment.Name,
	})
	if err != nil {
		return err
	}

	log.Info("Waiting for all pods to be running")
	err = pods.WatchAndWaitPodContainerRunning(client, clusterName, namespaceName, scaleDeployment)
	if err != nil {
		return err
	}

	log.Infof("Counting all pods running by image %s", image)
	countPods, err := pods.CountPodContainerRunningByImage(client, clusterName, namespaceName, image)
	if err != nil {
		return err
	}

	if expectedReplicas != countPods {
		err_msg := fmt.Sprintf("expected replica count: %d does not equal pod count: %d", expectedReplicas, countPods)
		return errors.New(err_msg)
	}

	return err
}

func VerifyDeploymentRolloutHistory(client *rancher.Client, clusterID, namespaceName string, deploymentName string, expectedRevision string) error {
	var wranglerContext *wrangler.Context
	var err error

	err = charts.WatchAndWaitDeployments(client, clusterID, namespaceName, metav1.ListOptions{
		FieldSelector: "metadata.name=" + deploymentName,
	})
	if err != nil {
		return err
	}

	wranglerContext = client.WranglerContext
	if clusterID != "local" {
		wranglerContext, err = client.WranglerContext.DownStreamClusterWranglerContext(clusterID)
		if err != nil {
			return err
		}
	}

	latestDeployment, err := wranglerContext.Apps.Deployment().Get(namespaceName, deploymentName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	if latestDeployment.ObjectMeta.Annotations == nil {
		return errors.New("revision empty")
	}

	revision := latestDeployment.ObjectMeta.Annotations[revisionAnnotation]

	if revision != expectedRevision {
		return errors.New("revision not found")
	}

	return nil
}

func VerifyOrchestrationStatus(client *rancher.Client, clusterID, namespaceName string, deploymentName string, isPaused bool) error {
	var wranglerContext *wrangler.Context
	var err error

	err = charts.WatchAndWaitDeployments(client, clusterID, namespaceName, metav1.ListOptions{
		FieldSelector: "metadata.name=" + deploymentName,
	})
	if err != nil {
		return err
	}

	wranglerContext = client.WranglerContext
	if clusterID != "local" {
		wranglerContext, err = client.WranglerContext.DownStreamClusterWranglerContext(clusterID)
		if err != nil {
			return err
		}
	}

	latestDeployment, err := wranglerContext.Apps.Deployment().Get(namespaceName, deploymentName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	if isPaused && !latestDeployment.Spec.Paused {
		return errors.New("the orchestration is active")
	}

	if !isPaused && latestDeployment.Spec.Paused {
		return errors.New("the orchestration is paused")
	}

	return nil
}
