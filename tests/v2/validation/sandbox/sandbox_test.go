//go:build (validation || extended) && !infra.any && !infra.aks && !infra.eks && !infra.rke2k3s && !infra.gke && !infra.rke1 && !cluster.any && !cluster.custom && !cluster.nodedriver && !sanity && !stress

package sandbox

import (
	"testing"

	"github.com/rancher/rancher/tests/v2/validation/workloads"
	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	"github.com/rancher/shepherd/extensions/clusters"
	"github.com/rancher/shepherd/pkg/session"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type WorkloadTestSuite struct {
	suite.Suite
	client  *rancher.Client
	session *session.Session
	cluster *management.Cluster
}

func (w *WorkloadTestSuite) TearDownSuite() {
	w.session.Cleanup()
}

func (w *WorkloadTestSuite) SetupSuite() {
	w.session = session.NewSession()

	client, err := rancher.NewClient("", w.session)
	require.NoError(w.T(), err)

	w.client = client

	log.Info("Getting cluster name from the config file and append cluster details in connection")
	clusterName := client.RancherConfig.ClusterName
	require.NotEmptyf(w.T(), clusterName, "Cluster name to install should be set")

	clusterID, err := clusters.GetClusterIDByName(w.client, clusterName)
	require.NoError(w.T(), err, "Error getting cluster ID")

	w.cluster, err = w.client.Management.Cluster.ByID(clusterID)
	require.NoError(w.T(), err)
}

// NOTE: this is an example DO NOT MERGE
func (w *WorkloadTestSuite) TestDemoSubTestCase1() {
	//Do some test i.e provision a cluster for OS-checks, snapshot test, normal provisioning test... etc
	// ---
	// ---
	// ---
	//As part of the above test we now want to chain this with the network checks,
	subSession := w.session.NewSession()
	defer subSession.Cleanup()

	workloadTests := workloads.GetAllWorkloadTests()
	for _, workloadTest := range workloadTests {
		w.Suite.Run(workloadTest.Name, func() {
			err := workloadTest.TestFunc(w.client, w.cluster.ID)
			require.NoError(w.T(), err)
		})
	}
}

// NOTE: this is an example DO NOT MERGE EXAMPLE
func (w *WorkloadTestSuite) TestDemoSubTestCase2() {
	//Do some test i.e provision a cluster for OS-checks, snapshot test, normal provisioning test... etc
	// ---
	// ---
	// ---
	//As part of the above test we now want to chain this with the network checks,
	subSession := w.session.NewSession()
	defer subSession.Cleanup()

	workloadTests := []struct {
		name     string
		testFunc workloads.WorkloadTestFunc
	}{
		{"WorkloadDeploymentTest", workloads.CreateDeploymentTest},
		{"WorkloadSideKickTest", workloads.DeploymentSideKickTest},
		{"WorkloadDaemonSetTest", workloads.CreateDaemonSetTest},
		{"WorkloadCronjobTest", workloads.CreateCronjobTest},
		{"WorkloadStatefulsetTest", workloads.CreateStatefulsetTest},
		{"WorkloadUpgradeTest", workloads.DeploymentUpgradeTest},
		{"WorkloadPodScaleUpTest", workloads.DeploymentPodScaleUpTest},
		{"WorkloadPodScaleDownTest", workloads.DeploymentPodScaleDownTest},
		{"WorkloadPauseOrchestrationTest", workloads.DeploymentPauseOrchestrationTest},
	}

	for _, workloadTest := range workloadTests {
		w.Suite.Run(workloadTest.name, func() {
			err := workloadTest.testFunc(w.client, w.cluster.ID)
			require.NoError(w.T(), err)
		})
	}
}

func TestSandboxSuite(t *testing.T) {
	suite.Run(t, new(WorkloadTestSuite))
}
