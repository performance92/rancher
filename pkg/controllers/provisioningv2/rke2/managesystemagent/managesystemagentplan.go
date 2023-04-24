package managesystemagent

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/Masterminds/semver/v3"
	"github.com/rancher/fleet/pkg/apis/fleet.cattle.io/v1alpha1"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	rancherv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/controllers/provisioningv2/rke2"
	namespaces "github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/provisioningv2/image"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func (h *handler) OnChangeInstallSUC(cluster *rancherv1.Cluster, status rancherv1.ClusterStatus) ([]runtime.Object, rancherv1.ClusterStatus, error) {
	if cluster.Spec.RKEConfig == nil {
		return nil, status, nil
	}

	currentVersion, err := semver.NewVersion(cluster.Spec.KubernetesVersion)
	if err != nil {
		return nil, status, err
	}

	// indicate to the SUC chart if we want to
	// install PodSecurityPolicy manifests
	pspEnabled := false
	if currentVersion.LessThan(Kubernetes125) {
		pspEnabled = true
	}

	// we must limit the output of name.SafeConcatName to at most 48 characters because
	// a) the chart release name cannot exceed 53 characters, and
	// b) upon creation of this resource the prefix 'mcc-' will be added to the release name, hence the limiting to 48 characters
	mcc := &v3.ManagedChart{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: cluster.Namespace,
			Name:      rke2.SafeConcatName(48, cluster.Name, "managed", "system-upgrade-controller"),
		},
		Spec: v3.ManagedChartSpec{
			DefaultNamespace: namespaces.System,
			RepoName:         "rancher-charts",
			Chart:            "system-upgrade-controller",
			Version:          settings.SystemUpgradeControllerChartVersion.Get(),
			Values: &v1alpha1.GenericMap{
				Data: map[string]interface{}{
					"global": map[string]interface{}{
						"cattle": map[string]interface{}{
							"systemDefaultRegistry": image.GetPrivateRepoURLFromCluster(cluster),
							"psp": map[string]interface{}{
								"enabled": pspEnabled,
							},
						},
					},
				},
			},
			Targets: []v1alpha1.BundleTarget{
				{
					ClusterName: cluster.Name,
					ClusterSelector: &metav1.LabelSelector{
						MatchExpressions: []metav1.LabelSelectorRequirement{
							{
								Key:      "provisioning.cattle.io/unmanaged-system-agent",
								Operator: metav1.LabelSelectorOpDoesNotExist,
							},
						},
					},
				},
			},
		},
	}

	return []runtime.Object{
		mcc,
	}, status, nil
}

type SUCMetadata struct {
	PspEnabled bool
}

// syncSystemUpgradeControllerStatus queries the managed system-upgrade-controller chart and determines if it is properly configured for a given
// version of Kubernetes. It applies a condition onto the control-plane object to be used by the planner when handling Kubernetes upgrades.
func (h *handler) syncSystemUpgradeControllerStatus(obj *rkev1.RKEControlPlane, status rkev1.RKEControlPlaneStatus) (rkev1.RKEControlPlaneStatus, error) {
	// perform the same name limiting as in the OnChangeInstallSUC and the managedchart controller
	bundleName := rke2.SafeConcatName(rke2.MaxHelmReleaseNameLength, "mcc", rke2.SafeConcatName(48, obj.Name, "managed", "system-upgrade-controller"))
	sucBundle, err := h.bundles.Get(obj.Namespace, bundleName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			// if we couldn't find the bundle then we know it's not ready
			rke2.SystemUpgradeControllerReady.SetError(&status, fmt.Sprintf("unable to find bundle %s", bundleName), err)
			// don't return the error, otherwise the status won't be set to 'false'
			err = nil
		}
		logrus.Errorf("[managesystemagentplan] rkecluster %s/%s: error encountered while retrieving bundle %s: %v", obj.Namespace, obj.Name, bundleName, err)
		return status, err
	}
	// determine if the SUC deployment has been rolled out fully, and if there were any errors encountered
	if sucBundle.Status.Summary.Ready != sucBundle.Status.Summary.DesiredReady {
		if sucBundle.Status.Summary.ErrApplied != 0 && len(sucBundle.Status.Summary.NonReadyResources) > 0 {
			nonReady := sucBundle.Status.Summary.NonReadyResources
			rke2.SystemUpgradeControllerReady.Reason(&status, fmt.Sprintf("Error Encountered Waiting for System Upgrade Controller Deployment To Roll Out: %s", nonReady[0].Message))
			rke2.SystemUpgradeControllerReady.Unknown(&status)
			return status, nil
		}
		rke2.SystemUpgradeControllerReady.Reason(&status, "Waiting for System Upgrade Controller Deployment roll out")
		rke2.SystemUpgradeControllerReady.Unknown(&status)
		return status, nil
	}

	// we need to look at the values yaml content to determine if PSPs are enabled
	// or disabled. We need to wait until SUC is redeployed if we don't see any helm values, as
	// we expect the PSP value to be explicitly defined as either true or false
	valuesYamlAvailable := sucBundle.Spec.Helm != nil && sucBundle.Spec.Helm.Values != nil
	if !valuesYamlAvailable {
		rke2.SystemUpgradeControllerReady.Reason(&status, "Waiting for Upgraded System Upgrade Controller Deployment")
		rke2.SystemUpgradeControllerReady.Unknown(&status)
		return status, nil
	}

	// look through the values yaml content to determine if 'psp: enabled: true'
	data := sucBundle.Spec.Helm.Values.Data
	global, ok := data["global"].(map[string]interface{})
	if !ok {
		logrus.Debugf("[managesystemagentplan] rkecluster %s/%s: bundle %s did not have global field", obj.Namespace, obj.Name, bundleName)
		rke2.SystemUpgradeControllerReady.Reason(&status, "Waiting for System Upgrade Controller Bundle Update")
		rke2.SystemUpgradeControllerReady.Unknown(&status)
		return status, nil
	}

	cattle, ok := global["cattle"].(map[string]interface{})
	if !ok {
		logrus.Debugf("[managesystemagentplan] rkecluster %s/%s: bundle %s did not have global.cattle field", obj.Namespace, obj.Name, bundleName)
		rke2.SystemUpgradeControllerReady.Reason(&status, "Waiting for System Upgrade Controller Bundle Update")
		rke2.SystemUpgradeControllerReady.Unknown(&status)
		return status, nil
	}

	psp, ok := cattle["psp"].(map[string]interface{})
	if !ok {
		logrus.Debugf("[managesystemagentplan] rkecluster %s/%s: bundle %s did not have global.cattle.psp field", obj.Namespace, obj.Name, bundleName)
		rke2.SystemUpgradeControllerReady.Reason(&status, "Waiting for System Upgrade Controller Bundle Update")
		rke2.SystemUpgradeControllerReady.Unknown(&status)
		return status, nil
	}

	currentVersion, err := semver.NewVersion(obj.Spec.KubernetesVersion)
	if err != nil {
		return status, err
	}

	// we only want to block an upgrade if PSPs are enabled AND we are on
	// a version greater than or equal to 1.25.
	enabled, ok := psp["enabled"].(bool)
	if !ok {
		rke2.SystemUpgradeControllerReady.Reason(&status, "Waiting for System Upgrade Controller Bundle Update")
		rke2.SystemUpgradeControllerReady.Unknown(&status)
		return status, nil
	}

	if !currentVersion.LessThan(Kubernetes125) && enabled {
		logrus.Debugf("[managesystemagentplan] rkecluster %s/%s: bundle %s still has SUC PSPs enabled", obj.Namespace, obj.Name, bundleName)
		rke2.SystemUpgradeControllerReady.Reason(&status, "System Upgrade Controller Not Ready")
		rke2.SystemUpgradeControllerReady.False(&status)
		return status, nil
	}

	metadata, err := json.Marshal(SUCMetadata{
		PspEnabled: enabled,
	})
	if err != nil {
		logrus.Errorf("[managesystemagentplan] rkecluster %s/%s: error while marshaling SUC Metadata: %v", obj.Namespace, obj.Name, err)
		return status, err
	}

	rke2.SystemUpgradeControllerReady.Message(&status, base64.StdEncoding.EncodeToString(metadata))
	rke2.SystemUpgradeControllerReady.True(&status)
	return status, nil
}
