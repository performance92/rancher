package observability

import (
	"context"
	"strings"

	"testing"

	provv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/tests/v2/actions/charts"
	"github.com/rancher/rancher/tests/v2/actions/fleet"
	"github.com/rancher/rancher/tests/v2/actions/kubeapi/namespaces"
	kubeprojects "github.com/rancher/rancher/tests/v2/actions/kubeapi/projects"
	"github.com/rancher/rancher/tests/v2/actions/observability"
	"github.com/rancher/rancher/tests/v2/actions/projects"
	"github.com/rancher/rancher/tests/v2/actions/rbac"
	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	steveV1 "github.com/rancher/shepherd/clients/rancher/v1"
	extencharts "github.com/rancher/shepherd/extensions/charts"
	"github.com/rancher/shepherd/extensions/clusters"
	extensionscluster "github.com/rancher/shepherd/extensions/clusters"
	"github.com/rancher/shepherd/extensions/users"
	"github.com/rancher/shepherd/extensions/workloads/pods"
	"github.com/rancher/shepherd/pkg/config"
	"github.com/rancher/shepherd/pkg/session"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	uiExtensionsRepo = "https://github.com/rancher/ui-plugin-charts"
	uiGitBranch      = "main"
	rancherUIPlugins = "rancher-ui-plugins"
)

const (
	project                 = "management.cattle.io.project"
	rancherPartnerCharts    = "rancher-partner-charts"
	systemProject           = "System"
	localCluster            = "local"
	stackStateConfigFileKey = "stackstateConfigs"
)

type StackStateRBACTestSuite struct {
	suite.Suite
	client                        *rancher.Client
	session                       *session.Session
	cluster                       *management.Cluster
	projectID                     string
	stackstateAgentInstallOptions *charts.InstallOptions
	stackstateConfigs             observability.StackStateConfigs
}

func (rb *StackStateRBACTestSuite) TearDownSuite() {
	rb.session.Cleanup()
}

func (rb *StackStateRBACTestSuite) SetupSuite() {
	testSession := session.NewSession()
	rb.session = testSession

	client, err := rancher.NewClient("", testSession)
	require.NoError(rb.T(), err)

	rb.client = client

	log.Info("Getting cluster name from the config file and append cluster details in rb")
	clusterName := client.RancherConfig.ClusterName
	require.NotEmptyf(rb.T(), clusterName, "Cluster name to install should be set")
	cluster, err := clusters.NewClusterMeta(rb.client, clusterName)
	require.NoError(rb.T(), err)
	rb.cluster, err = rb.client.Management.Cluster.ByID(cluster.ID)
	require.NoError(rb.T(), err)

	log.Info("Creating a project and namespace for the chart to be installed in.")

	projectTemplate := kubeprojects.NewProjectTemplate(cluster.ID)
	projectTemplate.Name = charts.StackstateNamespace
	project, err := client.Steve.SteveType(project).Create(projectTemplate)
	require.NoError(rb.T(), err)
	rb.projectID = project.ID

	_, err = namespaces.CreateNamespace(client, cluster.ID, project.Name, charts.StackstateNamespace, "", map[string]string{}, map[string]string{})
	require.NoError(rb.T(), err)

	log.Info("Verifying if the ui plugin repository for ui extensions exists.")
	_, err = rb.client.Catalog.ClusterRepos().Get(context.TODO(), rancherUIPlugins, meta.GetOptions{})

	if k8sErrors.IsNotFound(err) {
		err = observability.CreateExtensionsRepo(rb.client, rancherUIPlugins, uiExtensionsRepo, uiGitBranch)
	}
	require.NoError(rb.T(), err)

	var stackstateConfigs observability.StackStateConfigs
	config.LoadConfig(stackStateConfigFileKey, &stackstateConfigs)
	rb.stackstateConfigs = stackstateConfigs

	log.Info("Crete a node driver with stackstate extensions ui to whitelist stackstate URL")
	_, err = client.Management.NodeDriver.ByID(observability.StackstateName)
	if strings.Contains(err.Error(), "Not Found") {
		err = observability.InstallNodeDriver(rb.client, []string{rb.stackstateConfigs.Url})
	}
	require.NoError(rb.T(), err)

	rb.T().Log("Checking if the stack state CRD is installed.")
	crdsExists, err := rb.client.Steve.SteveType(observability.ApiExtenisonsCRD).ByID(observability.ObservabilitySteveType)
	if crdsExists == nil && strings.Contains(err.Error(), "Not Found") {
		err = observability.InstallStackstateCRD(rb.client)
	}
	require.NoError(rb.T(), err)

	client, err = client.ReLogin()
	require.NoError(rb.T(), err)

	rb.T().Log("Checking if the stack state extension is already installed.")
	initialStackstateExtension, err := extencharts.GetChartStatus(client, localCluster, charts.StackstateExtensionNamespace, charts.StackstateExtensionsName)
	require.NoError(rb.T(), err)

	if !initialStackstateExtension.IsAlreadyInstalled {
		latestUIPluginVersion, err := rb.client.Catalog.GetLatestChartVersion(charts.StackstateExtensionsName, charts.UIPluginName)
		require.NoError(rb.T(), err)

		rb.T().Log("Installing stackstate ui extensions")
		extensionOptions := &charts.ExtensionOptions{
			ChartName:   charts.StackstateExtensionsName,
			ReleaseName: charts.StackstateExtensionsName,
			Version:     latestUIPluginVersion,
		}

		err = charts.InstallStackstateExtension(client, extensionOptions)
		require.NoError(rb.T(), err)
	}

	log.Info("Adding stackstate extension configuration.")

	steveAdminClient, err := client.Steve.ProxyDownstream(localCluster)
	require.NoError(rb.T(), err)

	crdConfig := observability.NewStackstateCRDConfiguration(charts.StackstateNamespace, observability.StackstateName, rb.stackstateConfigs)
	crd, err := steveAdminClient.SteveType(charts.StackstateCRD).Create(crdConfig)
	require.NoError(rb.T(), err, "Unable to install stackstate CRD configuration.")

	config, err := steveAdminClient.SteveType(charts.StackstateCRD).ByID(crd.ID)
	require.NoError(rb.T(), err)
	require.Equal(rb.T(), observability.StackstateName, config.ObjectMeta.Name, "Stackstate configuration name differs.")

	latestSSVersion, err := rb.client.Catalog.GetLatestChartVersion(charts.StackstateK8sAgent, rancherPartnerCharts)
	require.NoError(rb.T(), err)
	rb.stackstateAgentInstallOptions = &charts.InstallOptions{
		Cluster:   cluster,
		Version:   latestSSVersion,
		ProjectID: rb.projectID,
	}
}

func (rb *StackStateRBACTestSuite) TestClusterOwnerInstallStackstate() {
	subSession := rb.session.NewSession()
	defer subSession.Cleanup()

	client, err := rb.client.WithSession(subSession)
	require.NoError(rb.T(), err)

	initialStackstateAgent, err := extencharts.GetChartStatus(client, rb.cluster.ID, charts.StackstateNamespace, charts.StackstateK8sAgent)
	require.NoError(rb.T(), err)

	if initialStackstateAgent.IsAlreadyInstalled {
		rb.T().Skip("Stack state agent is already installed, skipping the tests.")
	}

	var newUser *management.User
	user, err := users.CreateUserWithRole(rb.client, users.UserConfig(), rbac.StandardUser.String())
	require.NoError(rb.T(), err)
	newUser = user
	rb.T().Logf("Created user: %v", newUser.Username)

	standardClient, err := rb.client.AsUser(user)
	require.NoError(rb.T(), err)

	err = users.AddClusterRoleToUser(rb.client, rb.cluster, user, rbac.ClusterOwner.String(), nil)
	require.NoError(rb.T(), err)

	systemProject, err := projects.GetProjectByName(client, rb.cluster.ID, systemProject)
	require.NoError(rb.T(), err)
	require.NotNil(rb.T(), systemProject.ID, "System project is nil.")
	systemProjectID := strings.Split(systemProject.ID, ":")[1]

	err = charts.InstallStackstateAgentChart(standardClient, rb.stackstateAgentInstallOptions, rb.stackstateConfigs.ClusterApiKey, rb.stackstateConfigs.Url, systemProjectID)
	require.NoError(rb.T(), err)
	log.Info("Stackstate agent chart installed successfully")

	rb.T().Log("Verifying the deployments of stackstate agent chart to have expected number of available replicas")
	err = extencharts.WatchAndWaitDeployments(client, rb.cluster.ID, charts.StackstateNamespace, meta.ListOptions{})
	require.NoError(rb.T(), err)

	rb.T().Log("Verifying the daemonsets of stackstate agent chart to have expected number of available replicas nodes")
	err = extencharts.WatchAndWaitDaemonSets(client, rb.cluster.ID, charts.StackstateNamespace, meta.ListOptions{})
	require.NoError(rb.T(), err)

	clusterObject, _, _ := extensionscluster.GetProvisioningClusterByName(rb.client, rb.client.RancherConfig.ClusterName, fleet.Namespace)
	if clusterObject != nil {
		status := &provv1.ClusterStatus{}
		err := steveV1.ConvertToK8sType(clusterObject.Status, status)
		require.NoError(rb.T(), err)

		podErrors := pods.StatusPods(client, status.ClusterName)
		require.Empty(rb.T(), podErrors)
	}
}

func (rb *StackStateRBACTestSuite) TestMembersCannotInstallStackstate() {
	subSession := rb.session.NewSession()
	defer subSession.Cleanup()

	client, err := rb.client.WithSession(subSession)
	require.NoError(rb.T(), err)

	initialStackstateAgent, err := extencharts.GetChartStatus(client, rb.cluster.ID, charts.StackstateNamespace, charts.StackstateK8sAgent)
	require.NoError(rb.T(), err)

	if initialStackstateAgent.IsAlreadyInstalled {
		rb.T().Skip("Stack state agent is already installed, skipping the tests.")
	}

	tests := []struct {
		name   string
		role   rbac.Role
		member string
	}{
		{"Cluster Member", rbac.ClusterMember, rbac.StandardUser.String()},
		{"Project Owner", rbac.ProjectOwner, rbac.StandardUser.String()},
		{"Project Member", rbac.ProjectMember, rbac.StandardUser.String()},
	}

	for _, tt := range tests {
		var newUser *management.User
		user, err := users.CreateUserWithRole(rb.client, users.UserConfig(), tt.member)
		require.NoError(rb.T(), err)
		newUser = user
		rb.T().Logf("Created user: %v", newUser.Username)

		standardClient, err := rb.client.AsUser(user)
		require.NoError(rb.T(), err)

		if strings.Contains(tt.role.String(), "project") {
			stackstateProject, err := client.Management.Project.ByID(rb.projectID)
			require.NoError(rb.T(), err)
			err = users.AddProjectMember(rb.client, stackstateProject, user, tt.role.String(), nil)
			require.NoError(rb.T(), err)
		} else {
			err := users.AddClusterRoleToUser(rb.client, rb.cluster, user, tt.role.String(), nil)
			require.NoError(rb.T(), err)
		}

		systemProject, err := projects.GetProjectByName(client, rb.cluster.ID, systemProject)
		require.NoError(rb.T(), err)
		require.NotNil(rb.T(), systemProject.ID, "System project is nil.")
		systemProjectID := strings.Split(systemProject.ID, ":")[1]

		err = charts.InstallStackstateAgentChart(standardClient, rb.stackstateAgentInstallOptions, rb.stackstateConfigs.ClusterApiKey, rb.stackstateConfigs.Url, systemProjectID)
		require.Error(rb.T(), err)
		k8sErrors.IsForbidden(err)
		log.Info("Unable to install Stackstate agent chart as " + tt.name)
	}
}

func TestStackStateRBACTestSuite(t *testing.T) {
	suite.Run(t, new(StackStateRBACTestSuite))
}
