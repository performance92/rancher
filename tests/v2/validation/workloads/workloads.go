package workloads

import (
	"errors"
	"fmt"

	projectsapi "github.com/rancher/rancher/tests/v2/actions/projects"
	"github.com/rancher/rancher/tests/v2/actions/workloads/cronjob"
	"github.com/rancher/rancher/tests/v2/actions/workloads/daemonset"
	deployment "github.com/rancher/rancher/tests/v2/actions/workloads/deployment"
	"github.com/rancher/rancher/tests/v2/actions/workloads/pods"
	"github.com/rancher/rancher/tests/v2/actions/workloads/statefulset"
	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/extensions/charts"
	"github.com/rancher/shepherd/extensions/workloads"
	namegen "github.com/rancher/shepherd/pkg/namegenerator"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	nginxImageName  = "nginx"
	ubuntuImageName = "ubuntu"
	redisImageName  = "redis"
)

type WorkloadTestFunc func(client *rancher.Client, clusterID string) error

type WorkloadTest struct {
	Name     string
	TestFunc WorkloadTestFunc
}

func GetAllWorkloadTests() []WorkloadTest {
	tests := []WorkloadTest{
		{"WorkloadDeploymentTest", CreateDeploymentTest},
		{"WorkloadSideKickTest", DeploymentSideKickTest},
		{"WorkloadDaemonSetTest", CreateDaemonSetTest},
		{"WorkloadCronjobTest", CreateCronjobTest},
		{"WorkloadStatefulsetTest", CreateStatefulsetTest},
		{"WorkloadUpgradeTest", DeploymentUpgradeTest},
		{"WorkloadPodScaleUpTest", DeploymentPodScaleUpTest},
		{"WorkloadPodScaleDownTest", DeploymentPodScaleDownTest},
		{"WorkloadPauseOrchestrationTest", DeploymentPauseOrchestrationTest},
	}

	return tests
}

func CreateDeploymentTest(client *rancher.Client, clusterID string) error {
	log.Info("Creating new project and namespace")
	_, namespace, err := projectsapi.CreateProjectAndNamespace(client, clusterID)
	if err != nil {
		return err
	}

	log.Info("Creating new deployment")
	_, err = deployment.CreateDeployment(client, clusterID, namespace.Name, 1, "", "", false, false, true)

	return err
}

func CreateDaemonSetTest(client *rancher.Client, clusterID string) error {
	log.Info("Creating new project and namespace")
	_, namespace, err := projectsapi.CreateProjectAndNamespace(client, clusterID)
	if err != nil {
		return err
	}

	log.Info("Creating new deamonset")
	createdDaemonset, err := daemonset.CreateDaemonset(client, clusterID, namespace.Name, 1, "", "", false, false)
	if err != nil {
		return err
	}

	log.Info("Waiting deamonset comes up active")
	err = charts.WatchAndWaitDaemonSets(client, clusterID, namespace.Name, metav1.ListOptions{
		FieldSelector: "metadata.name=" + createdDaemonset.Name,
	})

	return err
}

func CreateCronjobTest(client *rancher.Client, clusterID string) error {
	log.Info("Creating new project and namespace")
	_, namespace, err := projectsapi.CreateProjectAndNamespace(client, clusterID)
	if err != nil {
		return err
	}

	containerName := namegen.AppendRandomString("test-container")
	pullPolicy := corev1.PullAlways

	containerTemplate := workloads.NewContainer(
		containerName,
		nginxImageName,
		pullPolicy,
		[]corev1.VolumeMount{},
		[]corev1.EnvFromSource{},
		nil,
		nil,
		nil,
	)
	podTemplate := workloads.NewPodTemplate(
		[]corev1.Container{containerTemplate},
		[]corev1.Volume{},
		[]corev1.LocalObjectReference{},
		nil,
		nil,
	)

	log.Info("Creating new cronjob")
	cronJobTemplate, err := cronjob.CreateCronjob(client, clusterID, namespace.Name, "*/1 * * * *", podTemplate)
	if err != nil {
		return err
	}

	log.Info("Waiting cronjob comes up active")
	err = cronjob.WatchAndWaitCronjob(client, clusterID, namespace.Name, cronJobTemplate)

	return err
}

func CreateStatefulsetTest(client *rancher.Client, clusterID string) error {
	log.Info("Creating new project and namespace")
	_, namespace, err := projectsapi.CreateProjectAndNamespace(client, clusterID)
	if err != nil {
		return err
	}

	containerName := namegen.AppendRandomString("test-container")
	pullPolicy := corev1.PullAlways

	containerTemplate := workloads.NewContainer(
		containerName,
		nginxImageName,
		pullPolicy,
		[]corev1.VolumeMount{},
		[]corev1.EnvFromSource{},
		nil,
		nil,
		nil,
	)
	podTemplate := workloads.NewPodTemplate(
		[]corev1.Container{containerTemplate},
		[]corev1.Volume{},
		[]corev1.LocalObjectReference{},
		nil,
		nil,
	)

	log.Info("Creating new statefulset")
	statefulsetTemplate, err := statefulset.CreateStatefulset(client, clusterID, namespace.Name, podTemplate, 1)
	if err != nil {
		return err
	}

	log.Info("Waiting statefulset comes up active")
	err = charts.WatchAndWaitStatefulSets(client, clusterID, namespace.Name, metav1.ListOptions{
		FieldSelector: "metadata.name=" + statefulsetTemplate.Name,
	})

	return err
}

func DeploymentSideKickTest(client *rancher.Client, clusterID string) error {
	log.Info("Creating new project and namespace")
	_, namespace, err := projectsapi.CreateProjectAndNamespace(client, clusterID)
	if err != nil {
		return err
	}

	log.Info("Creating new deployment")
	createdDeployment, err := deployment.CreateDeployment(client, clusterID, namespace.Name, 1, "", "", false, false, true)
	if err != nil {
		return err
	}

	log.Info("Waiting for all pods to be running")
	err = pods.WatchAndWaitPodContainerRunning(client, clusterID, namespace.Name, createdDeployment)
	if err != nil {
		return err
	}

	containerName := namegen.AppendRandomString("update-test-container")
	newContainerTemplate := workloads.NewContainer(containerName,
		redisImageName,
		corev1.PullAlways,
		[]corev1.VolumeMount{},
		[]corev1.EnvFromSource{},
		nil,
		nil,
		nil,
	)

	createdDeployment.Spec.Template.Spec.Containers = append(createdDeployment.Spec.Template.Spec.Containers, newContainerTemplate)

	log.Info("Updating image deployment")
	updatedDeployment, err := deployment.UpdateDeployment(client, clusterID, namespace.Name, createdDeployment, true)
	if err != nil {
		return err
	}

	log.Info("Waiting deployment comes up active")
	err = charts.WatchAndWaitDeployments(client, clusterID, namespace.Name, metav1.ListOptions{
		FieldSelector: "metadata.name=" + updatedDeployment.Name,
	})
	if err != nil {
		return err
	}

	log.Info("Waiting for all pods to be running")
	err = pods.WatchAndWaitPodContainerRunning(client, clusterID, namespace.Name, updatedDeployment)

	return err
}

func DeploymentUpgradeTest(client *rancher.Client, clusterID string) error {
	log.Info("Creating new project and namespace")
	_, namespace, err := projectsapi.CreateProjectAndNamespace(client, clusterID)
	if err != nil {
		return err
	}

	log.Info("Creating new deployment")
	upgradeDeployment, err := deployment.CreateDeployment(client, clusterID, namespace.Name, 2, "", "", false, false, true)
	if err != nil {
		return err
	}

	err = deployment.VerifyDeploymentUpgrade(client, clusterID, namespace.Name, upgradeDeployment, "1", nginxImageName, 2)
	if err != nil {
		return err
	}

	containerName := namegen.AppendRandomString("update-test-container")
	newContainerTemplate := workloads.NewContainer(containerName,
		redisImageName,
		corev1.PullAlways,
		[]corev1.VolumeMount{},
		[]corev1.EnvFromSource{},
		nil,
		nil,
		nil,
	)
	upgradeDeployment.Spec.Template.Spec.Containers = []corev1.Container{newContainerTemplate}

	log.Info("Updating deployment")
	upgradeDeployment, err = deployment.UpdateDeployment(client, clusterID, namespace.Name, upgradeDeployment, true)
	if err != nil {
		return err
	}

	err = deployment.VerifyDeploymentUpgrade(client, clusterID, namespace.Name, upgradeDeployment, "2", redisImageName, 2)
	if err != nil {
		return err
	}

	containerName = namegen.AppendRandomString("update-test-container-two")
	newContainerTemplate = workloads.NewContainer(containerName,
		ubuntuImageName,
		corev1.PullAlways,
		[]corev1.VolumeMount{},
		[]corev1.EnvFromSource{},
		nil,
		nil,
		nil,
	)
	newContainerTemplate.TTY = true
	newContainerTemplate.Stdin = true
	upgradeDeployment.Spec.Template.Spec.Containers = []corev1.Container{newContainerTemplate}

	log.Info("Updating deployment")
	_, err = deployment.UpdateDeployment(client, clusterID, namespace.Name, upgradeDeployment, true)
	if err != nil {
		return err
	}

	err = deployment.VerifyDeploymentUpgrade(client, clusterID, namespace.Name, upgradeDeployment, "3", ubuntuImageName, 2)
	if err != nil {
		return err
	}

	log.Info("Rollback deployment")
	logRollback, err := deployment.RollbackDeployment(client, clusterID, namespace.Name, upgradeDeployment.Name, 1)
	if err != nil {
		return err
	}
	if logRollback == "" {
		return err
	}

	err = deployment.VerifyDeploymentUpgrade(client, clusterID, namespace.Name, upgradeDeployment, "4", nginxImageName, 2)
	if err != nil {
		return err
	}

	log.Info("Rollback deployment")
	logRollback, err = deployment.RollbackDeployment(client, clusterID, namespace.Name, upgradeDeployment.Name, 2)
	if err != nil {
		return err
	}
	if logRollback == "" {
		return err
	}

	err = deployment.VerifyDeploymentUpgrade(client, clusterID, namespace.Name, upgradeDeployment, "5", redisImageName, 2)
	if err != nil {
		return err
	}

	log.Info("Rollback deployment")
	logRollback, err = deployment.RollbackDeployment(client, clusterID, namespace.Name, upgradeDeployment.Name, 3)
	if err != nil {
		return err
	}
	if logRollback == "" {
		return err
	}

	err = deployment.VerifyDeploymentUpgrade(client, clusterID, namespace.Name, upgradeDeployment, "6", ubuntuImageName, 2)
	if err != nil {
		return err
	}

	return err
}

func DeploymentPodScaleUpTest(client *rancher.Client, clusterID string) error {
	log.Info("Creating new project and namespace")
	_, namespace, err := projectsapi.CreateProjectAndNamespace(client, clusterID)
	if err != nil {
		return err
	}

	log.Info("Creating new deployment")
	scaleUpDeployment, err := deployment.CreateDeployment(client, clusterID, namespace.Name, 1, "", "", false, false, true)
	if err != nil {
		return err
	}

	err = deployment.VerifyDeploymentScale(client, clusterID, namespace.Name, scaleUpDeployment, nginxImageName, 1)
	if err != nil {
		return err
	}

	replicas := int32(2)
	scaleUpDeployment.Spec.Replicas = &replicas

	log.Info("Updating deployment replicas")
	scaleUpDeployment, err = deployment.UpdateDeployment(client, clusterID, namespace.Name, scaleUpDeployment, true)
	if err != nil {
		return err
	}

	err = deployment.VerifyDeploymentScale(client, clusterID, namespace.Name, scaleUpDeployment, nginxImageName, 2)
	if err != nil {
		return err
	}

	replicas = int32(3)
	scaleUpDeployment.Spec.Replicas = &replicas

	log.Info("Updating deployment replicas")
	scaleUpDeployment, err = deployment.UpdateDeployment(client, clusterID, namespace.Name, scaleUpDeployment, true)
	if err != nil {
		return err
	}

	err = deployment.VerifyDeploymentScale(client, clusterID, namespace.Name, scaleUpDeployment, nginxImageName, 3)
	if err != nil {
		return err
	}

	return err
}

func DeploymentPodScaleDownTest(client *rancher.Client, clusterID string) error {
	log.Info("Creating new project and namespace")
	_, namespace, err := projectsapi.CreateProjectAndNamespace(client, clusterID)
	if err != nil {
		return err
	}

	log.Info("Creating new deployment")
	scaleDownDeployment, err := deployment.CreateDeployment(client, clusterID, namespace.Name, 3, "", "", false, false, true)
	if err != nil {
		return err
	}

	err = deployment.VerifyDeploymentScale(client, clusterID, namespace.Name, scaleDownDeployment, nginxImageName, 3)
	if err != nil {
		return err
	}

	replicas := int32(2)
	scaleDownDeployment.Spec.Replicas = &replicas

	log.Info("Updating deployment replicas")
	scaleDownDeployment, err = deployment.UpdateDeployment(client, clusterID, namespace.Name, scaleDownDeployment, true)
	if err != nil {
		return err
	}

	err = deployment.VerifyDeploymentScale(client, clusterID, namespace.Name, scaleDownDeployment, nginxImageName, 2)
	if err != nil {
		return err
	}

	replicas = int32(1)
	scaleDownDeployment.Spec.Replicas = &replicas

	log.Info("Updating deployment replicas")
	scaleDownDeployment, err = deployment.UpdateDeployment(client, clusterID, namespace.Name, scaleDownDeployment, true)
	if err != nil {
		return err
	}

	err = deployment.VerifyDeploymentScale(client, clusterID, namespace.Name, scaleDownDeployment, nginxImageName, 1)
	if err != nil {
		return err
	}

	return err
}

func DeploymentPauseOrchestrationTest(client *rancher.Client, clusterID string) error {
	log.Info("Creating new project and namespace")
	_, namespace, err := projectsapi.CreateProjectAndNamespace(client, clusterID)
	if err != nil {
		return err
	}

	log.Info("Creating new deployment")
	pauseDeployment, err := deployment.CreateDeployment(client, clusterID, namespace.Name, 2, "", "", false, false, true)
	if err != nil {
		return err
	}

	err = deployment.VerifyDeploymentScale(client, clusterID, namespace.Name, pauseDeployment, nginxImageName, 2)
	if err != nil {
		return err
	}

	log.Info("Pausing orchestration")
	pauseDeployment.Spec.Paused = true
	pauseDeployment, err = deployment.UpdateDeployment(client, clusterID, namespace.Name, pauseDeployment, true)
	if err != nil {
		return err
	}

	log.Info("Verifying orchestration is paused")
	err = deployment.VerifyOrchestrationStatus(client, clusterID, namespace.Name, pauseDeployment.Name, true)
	if err != nil {
		return err
	}

	replicas := int32(3)
	pauseDeployment.Spec.Replicas = &replicas
	containerName := namegen.AppendRandomString("pause-redis-container")
	newContainerTemplate := workloads.NewContainer(containerName,
		redisImageName,
		corev1.PullAlways,
		[]corev1.VolumeMount{},
		[]corev1.EnvFromSource{},
		nil,
		nil,
		nil,
	)
	pauseDeployment.Spec.Template.Spec.Containers = []corev1.Container{newContainerTemplate}

	log.Info("Updating deployment image and replica")
	pauseDeployment, err = deployment.UpdateDeployment(client, clusterID, namespace.Name, pauseDeployment, true)
	if err != nil {
		return err
	}

	log.Info("Waiting for all pods to be running")
	err = pods.WatchAndWaitPodContainerRunning(client, clusterID, namespace.Name, pauseDeployment)
	if err != nil {
		return err
	}

	log.Info("Verifying that the deployment was not updated and the replica count was increased")
	log.Infof("Counting all pods running by image %s", nginxImageName)
	countPods, err := pods.CountPodContainerRunningByImage(client, clusterID, namespace.Name, nginxImageName)
	if err != nil {
		return err
	}

	if int(replicas) != countPods {
		err_msg := fmt.Sprintf("expected replica count: %d does not equal pod count: %d", int(replicas), countPods)
		return errors.New(err_msg)
	}

	log.Info("Activing orchestration")
	pauseDeployment.Spec.Paused = false
	pauseDeployment, err = deployment.UpdateDeployment(client, clusterID, namespace.Name, pauseDeployment, true)

	err = deployment.VerifyDeploymentScale(client, clusterID, namespace.Name, pauseDeployment, redisImageName, int(replicas))

	log.Info("Verifying orchestration is active")
	err = deployment.VerifyOrchestrationStatus(client, clusterID, namespace.Name, pauseDeployment.Name, false)
	if err != nil {
		return err
	}

	log.Infof("Counting all pods running by image %s", redisImageName)
	countPods, err = pods.CountPodContainerRunningByImage(client, clusterID, namespace.Name, redisImageName)
	if err != nil {
		return err
	}

	if int(replicas) != countPods {
		err_msg := fmt.Sprintf("expected replica count: %d does not equal pod count: %d", int(replicas), countPods)
		return errors.New(err_msg)
	}

	return err
}
