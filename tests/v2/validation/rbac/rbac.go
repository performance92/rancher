package rbac

import (
	"errors"
	"sort"
	"strings"
	"time"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	v1 "github.com/rancher/shepherd/clients/rancher/v1"
	"github.com/rancher/shepherd/extensions/clusters"
	"github.com/rancher/shepherd/extensions/defaults/stevetypes"
	"github.com/rancher/shepherd/extensions/namespaces"
	"github.com/rancher/shepherd/extensions/projects"
	"github.com/rancher/shepherd/extensions/workloads"
	namegen "github.com/rancher/shepherd/pkg/namegenerator"
	appv1 "k8s.io/api/apps/v1"
	coreV1 "k8s.io/api/core/v1"
	kwait "k8s.io/apimachinery/pkg/util/wait"
)

const (
	roleOwner                     = "cluster-owner"
	roleMember                    = "cluster-member"
	roleProjectOwner              = "project-owner"
	roleProjectMember             = "project-member"
	roleCustomManageProjectMember = "projectroletemplatebindings-manage"
	roleCustomCreateNS            = "create-ns"
	roleProjectReadOnly           = "read-only"
	restrictedAdmin               = "restricted-admin"
	standardUser                  = "user"
	pssRestrictedPolicy           = "restricted"
	pssBaselinePolicy             = "baseline"
	pssPrivilegedPolicy           = "privileged"
	psaWarn                       = "pod-security.kubernetes.io/warn"
	psaAudit                      = "pod-security.kubernetes.io/audit"
	psaEnforce                    = "pod-security.kubernetes.io/enforce"
	kubeConfigTokenSettingID      = "kubeconfig-default-token-ttl-minutes"
	psaRole                       = "updatepsa"
	defaultNamespace              = "fleet-default"
	isCattleLabeled               = true
)

func listProjects(client *rancher.Client, clusterID string) ([]string, error) {
	projectList, err := projects.GetProjectList(client, clusterID)
	if err != nil {
		return nil, err
	}

	projectNames := make([]string, len(projectList.Data))

	for idx, project := range projectList.Data {
		projectNames[idx] = project.Name
	}
	sort.Strings(projectNames)
	return projectNames, nil
}

func getNamespaces(steveclient *v1.Client) ([]string, error) {

	namespaceList, err := steveclient.SteveType(namespaces.NamespaceSteveType).List(nil)
	if err != nil {
		return nil, err
	}

	namespace := make([]string, len(namespaceList.Data))
	for idx, ns := range namespaceList.Data {
		namespace[idx] = ns.GetName()
	}
	sort.Strings(namespace)
	return namespace, nil
}

func deleteNamespace(namespaceID *v1.SteveAPIObject, steveclient *v1.Client) error {
	deletens := steveclient.SteveType(namespaces.NamespaceSteveType).Delete(namespaceID)
	return deletens
}

func createProject(client *rancher.Client, clusterID string) (*management.Project, error) {
	projectName := namegen.AppendRandomString("testproject-")
	projectConfig := &management.Project{
		ClusterID: clusterID,
		Name:      projectName,
	}
	createProject, err := client.Management.Project.Create(projectConfig)
	if err != nil {
		return nil, err
	}
	return createProject, nil
}

func getPSALabels(response *v1.SteveAPIObject, actualLabels map[string]string) map[string]string {
	expectedLabels := map[string]string{}

	for label := range response.Labels {
		if _, found := actualLabels[label]; found {
			expectedLabels[label] = actualLabels[label]
		}
	}
	return expectedLabels
}

func createDeploymentAndWait(steveclient *v1.Client, client *rancher.Client, clusterID string, containerName string, image string, namespaceName string) (*v1.SteveAPIObject, error) {
	deploymentName := namegen.AppendRandomString("rbac-")
	containerTemplate := workloads.NewContainer(containerName, image, coreV1.PullAlways, []coreV1.VolumeMount{}, []coreV1.EnvFromSource{}, nil, nil, nil)

	podTemplate := workloads.NewPodTemplate([]coreV1.Container{containerTemplate}, []coreV1.Volume{}, []coreV1.LocalObjectReference{}, nil)
	deployment := workloads.NewDeploymentTemplate(deploymentName, namespaceName, podTemplate, isCattleLabeled, nil)

	deploymentResp, err := steveclient.SteveType(workloads.DeploymentSteveType).Create(deployment)
	if err != nil {
		return nil, err
	}
	err = kwait.Poll(5*time.Second, 5*time.Minute, func() (done bool, err error) {
		deploymentResp, err := steveclient.SteveType(workloads.DeploymentSteveType).ByID(deployment.Namespace + "/" + deployment.Name)
		if err != nil {
			return false, err
		}
		deployment := &appv1.Deployment{}
		err = v1.ConvertToK8sType(deploymentResp.JSONResp, deployment)
		if err != nil {
			return false, err
		}
		status := deployment.Status.Conditions
		for _, statusCondition := range status {
			if strings.Contains(statusCondition.Message, "forbidden") {
				err = errors.New(statusCondition.Message)
				return false, err
			}
		}
		if *deployment.Spec.Replicas == deployment.Status.AvailableReplicas {
			return true, nil
		}
		return false, nil
	})
	return deploymentResp, err
}

func getAndConvertNamespace(namespace *v1.SteveAPIObject, steveAdminClient *v1.Client) (*coreV1.Namespace, error) {
	getNSSteveObject, err := steveAdminClient.SteveType(namespaces.NamespaceSteveType).ByID(namespace.ID)
	if err != nil {
		return nil, err
	}
	namespaceObj := &coreV1.Namespace{}
	err = v1.ConvertToK8sType(getNSSteveObject.JSONResp, namespaceObj)
	if err != nil {
		return nil, err
	}
	return namespaceObj, nil
}

func deleteLabels(labels map[string]string) {
	for label := range labels {
		if strings.Contains(label, psaWarn) || strings.Contains(label, psaAudit) || strings.Contains(label, psaEnforce) {
			delete(labels, label)
		}
	}
}

func convertSetting(globalSetting *v1.SteveAPIObject) (*v3.Setting, error) {
	updateSetting := &v3.Setting{}
	err := v1.ConvertToK8sType(globalSetting.JSONResp, updateSetting)
	if err != nil {
		return nil, err
	}
	return updateSetting, nil
}

func listGlobalSettings(steveclient *v1.Client) ([]string, error) {
	globalSettings, err := steveclient.SteveType(stevetypes.ManagementSetting).List(nil)
	if err != nil {
		return nil, err
	}

	settingsNameList := make([]string, len(globalSettings.Data))
	for idx, setting := range globalSettings.Data {
		settingsNameList[idx] = setting.Name
	}
	sort.Strings(settingsNameList)
	return settingsNameList, nil
}

func editGlobalSettings(steveclient *v1.Client, globalSetting *v1.SteveAPIObject, value string) (*v1.SteveAPIObject, error) {
	updateSetting, err := convertSetting(globalSetting)
	if err != nil {
		return nil, err
	}

	updateSetting.Value = value
	updateGlobalSetting, err := steveclient.SteveType(stevetypes.ManagementSetting).Update(globalSetting, updateSetting)
	if err != nil {
		return nil, err
	}
	return updateGlobalSetting, nil
}

func createRole(client *rancher.Client, context string, roleName string, rules []management.PolicyRule) (role *management.RoleTemplate, err error) {
	role, err = client.Management.RoleTemplate.Create(
		&management.RoleTemplate{
			Context: "cluster",
			Name:    "additional-psa-role",
			Rules:   rules,
		})
	return

}

func editPsactCluster(client *rancher.Client, clustername string, namespace string, psact string) (clusterType string, err error) {
	clusterID, err := clusters.GetClusterIDByName(client, clustername)
	if err != nil {
		return "", err
	}
	//Check if the downstream cluster is RKE2/K3S or RKE1
	if strings.Contains(clusterID, "c-m-") {
		clusterType = "RKE2K3S"
		clusterObj, existingSteveAPIObj, err := clusters.GetProvisioningClusterByName(client, clustername, namespace)
		if err != nil {
			return "", err
		}

		clusterObj.Spec.DefaultPodSecurityAdmissionConfigurationTemplateName = psact
		_, err = clusters.UpdateK3SRKE2Cluster(client, existingSteveAPIObj, clusterObj)
		if err != nil {
			return clusterType, err
		}
		updatedClusterObj, _, err := clusters.GetProvisioningClusterByName(client, clustername, namespace)
		if err != nil {
			return "", err
		}
		if updatedClusterObj.Spec.DefaultPodSecurityAdmissionConfigurationTemplateName != psact {
			errorMsg := "psact value was not changed, Expected: " + psact + ", Actual: " + updatedClusterObj.Spec.DefaultPodSecurityAdmissionConfigurationTemplateName
			return clusterType, errors.New(errorMsg)
		}
	} else {
		clusterType = "RKE"
		if psact == "" {
			psact = " "
		}
		existingCluster, err := client.Management.Cluster.ByID(clusterID)
		if err != nil {
			return "", err
		}

		updatedCluster := &management.Cluster{
			Name: existingCluster.Name,
			DefaultPodSecurityAdmissionConfigurationTemplateName: psact,
		}
		_, err = client.Management.Cluster.Update(existingCluster, updatedCluster)
		if err != nil {
			return clusterType, err
		}

		err = clusters.WaitForActiveRKE1Cluster(client, clusterID)
		if err != nil {
			return "", err
		}

		modifiedCluster, err := client.Management.Cluster.ByID(clusterID)
		if err != nil {
			return "", err
		}
		if psact == " " {
			psact = ""
		}
		if modifiedCluster.DefaultPodSecurityAdmissionConfigurationTemplateName != psact {
			errorMsg := "psact value was not changed, Expected: " + psact + ", Actual: " + modifiedCluster.DefaultPodSecurityAdmissionConfigurationTemplateName
			return clusterType, errors.New(errorMsg)
		}
	}
	return clusterType, nil
}
