package fleetcluster

import (
	"fmt"
	"testing"

	fleet "github.com/rancher/fleet/pkg/apis/fleet.cattle.io/v1alpha1"
	mgmt "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	mgmtv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/stretchr/testify/require"
)

var (
	errNotImplemented = fmt.Errorf("unimplemented")
	errNotFound       = fmt.Errorf("not found")

	builtinAffinity = corev1.Affinity{
		NodeAffinity: &corev1.NodeAffinity{
			PreferredDuringSchedulingIgnoredDuringExecution: []corev1.PreferredSchedulingTerm{
				{
					Weight: 1,
					Preference: corev1.NodeSelectorTerm{
						MatchExpressions: []corev1.NodeSelectorRequirement{
							{
								Key:      "fleet.cattle.io/agent",
								Operator: corev1.NodeSelectorOpIn,
								Values:   []string{"true"},
							},
						},
					},
				},
			},
		},
	}
	linuxAffinity = corev1.Affinity{
		NodeAffinity: &corev1.NodeAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
				NodeSelectorTerms: []corev1.NodeSelectorTerm{
					{
						MatchExpressions: []corev1.NodeSelectorRequirement{
							{
								Key:      "kubernetes.io/os",
								Operator: corev1.NodeSelectorOpIn,
								Values:   []string{"linux"},
							},
						},
					},
				},
			},
		},
	}
	resourceReq = &corev1.ResourceRequirements{
		Limits: corev1.ResourceList{
			corev1.ResourceCPU: resource.MustParse("1"),
		},
		Requests: corev1.ResourceList{
			corev1.ResourceMemory: resource.MustParse("1Gi"),
		},
	}
	tolerations = []corev1.Toleration{
		{
			Key:      "key",
			Operator: corev1.TolerationOpEqual,
			Value:    "value",
		},
	}
)

func TestClusterCustomization(t *testing.T) {
	require := require.New(t)

	h := &handler{
		clustersCache:     &fakeClusterCache{},
		getPrivateRepoURL: func(*v1.Cluster, *mgmt.Cluster) string { return "" },
	}

	cluster := &v1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-cluster", Namespace: "test-namespace",
		},
		Spec: v1.ClusterSpec{},
	}
	clusterStatus := v1.ClusterStatus{ClusterName: "cluster-name", ClientSecretName: "client-secret-name"}

	labels := map[string]string{"cluster-group": "cluster-group-name"}

	tests := []struct {
		name          string
		cluster       *v1.Cluster
		status        v1.ClusterStatus
		clustersCache mgmtv3.ClusterCache
		expectedFleet *fleet.Cluster
	}{
		{
			"cluster-has-no-customization",
			cluster,
			clusterStatus,
			newClusterCache(map[string]*mgmt.Cluster{
				"cluster-name": newMgmtCluster(
					"cluster-name",
					labels,
					mgmt.ClusterSpec{},
				),
			}),
			&fleet.Cluster{
				Spec: fleet.ClusterSpec{
					AgentAffinity: &builtinAffinity,
				},
			},
		},
		{
			"cluster-has-affinity-override",
			cluster,
			clusterStatus,
			newClusterCache(map[string]*mgmt.Cluster{
				"cluster-name": newMgmtCluster(
					"cluster-name",
					labels,
					mgmt.ClusterSpec{
						ClusterSpecBase: mgmt.ClusterSpecBase{
							FleetAgentDeploymentCustomization: &mgmt.AgentDeploymentCustomization{
								OverrideAffinity:             &linuxAffinity,
								OverrideResourceRequirements: &corev1.ResourceRequirements{},
								AppendTolerations:            []corev1.Toleration{},
							},
						},
					},
				),
			}),
			&fleet.Cluster{
				Spec: fleet.ClusterSpec{
					AgentAffinity:    &linuxAffinity,
					AgentResources:   &corev1.ResourceRequirements{},
					AgentTolerations: []corev1.Toleration{},
				},
			},
		},
		{
			"cluster-has-custom-tolerations-and-resources",
			cluster,
			clusterStatus,
			newClusterCache(map[string]*mgmt.Cluster{
				"cluster-name": newMgmtCluster(
					"cluster-name",
					labels,
					mgmt.ClusterSpec{
						ClusterSpecBase: mgmt.ClusterSpecBase{
							FleetAgentDeploymentCustomization: &mgmt.AgentDeploymentCustomization{
								OverrideAffinity:             nil,
								OverrideResourceRequirements: resourceReq,
								AppendTolerations:            tolerations,
							},
						},
					},
				),
			}),
			&fleet.Cluster{
				Spec: fleet.ClusterSpec{
					AgentAffinity:    &builtinAffinity,
					AgentResources:   resourceReq,
					AgentTolerations: tolerations,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h.clustersCache = tt.clustersCache
			objs, _, err := h.createCluster(tt.cluster, tt.status)
			require.Nil(err)
			require.NotNil(objs)

			require.Equal(1, len(objs))

			fleetCluster, ok := objs[0].(*fleet.Cluster)
			if !ok {
				t.Errorf("Expected fleet cluster, got %t", objs[0])
			}

			require.Equal(tt.expectedFleet.Spec.AgentAffinity, fleetCluster.Spec.AgentAffinity)
			require.Equal(tt.expectedFleet.Spec.AgentResources, fleetCluster.Spec.AgentResources)
			require.Equal(tt.expectedFleet.Spec.AgentTolerations, fleetCluster.Spec.AgentTolerations)
		})
	}

}

func TestCreateCluster(t *testing.T) {
	h := &handler{
		clustersCache:     &fakeClusterCache{},
		getPrivateRepoURL: func(*v1.Cluster, *mgmt.Cluster) string { return "" },
	}

	tests := []struct {
		name          string
		cluster       *v1.Cluster
		status        v1.ClusterStatus
		clustersCache mgmtv3.ClusterCache
		expectedLen   int
	}{
		{
			"cluster-has-no-cg",
			&v1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "test-namespace",
				},
				Spec: v1.ClusterSpec{},
			},
			v1.ClusterStatus{
				ClusterName:      "cluster-name",
				ClientSecretName: "client-secret-name",
			},

			newClusterCache(map[string]*mgmt.Cluster{
				"cluster-name": newMgmtCluster(
					"cluster-name",
					map[string]string{
						"cluster-group": "cluster-group-name",
					},
					mgmt.ClusterSpec{Internal: false},
				),
			}),
			1,
		},
		{
			"local-cluster-has-cg-has-label",
			&v1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "local-cluster",
					Namespace: "fleet-local",
				},
				Spec: v1.ClusterSpec{},
			},
			v1.ClusterStatus{
				ClusterName:      "local-cluster",
				ClientSecretName: "local-kubeconfig",
			},
			newClusterCache(map[string]*mgmt.Cluster{
				"local-cluster": newMgmtCluster(
					"local-cluster",
					map[string]string{
						"cluster-group": "default",
					},
					mgmt.ClusterSpec{Internal: true},
				),
			}),
			2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h.clustersCache = tt.clustersCache
			objs, _, err := h.createCluster(tt.cluster, tt.status)

			if objs == nil {
				t.Errorf("Expected non-nil objs: %v", err)
			}

			if err != nil {
				t.Errorf("Expected nil err")
			}

			if len(objs) != tt.expectedLen {
				t.Errorf("Expected %d objects, got %d", tt.expectedLen, len(objs))
			}
		})
	}

}

func newMgmtCluster(name string, labels map[string]string, spec mgmt.ClusterSpec) *mgmt.Cluster {
	spec.DisplayName = name
	mgmtCluster := &mgmt.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
		Spec: spec,
	}
	mgmt.ClusterConditionReady.SetStatus(mgmtCluster, "True")
	return mgmtCluster

}

// implements v3.ClusterCache
func newClusterCache(clusters map[string]*mgmt.Cluster) mgmtv3.ClusterCache {
	return &fakeClusterCache{
		clusters: clusters,
	}
}

type fakeClusterCache struct {
	clusters map[string]*mgmt.Cluster
}

func (f *fakeClusterCache) Get(name string) (*mgmt.Cluster, error) {
	if c, ok := f.clusters[name]; ok {
		return c, nil
	}
	return nil, errNotFound
}
func (f *fakeClusterCache) List(selector labels.Selector) ([]*mgmt.Cluster, error) {
	return nil, errNotImplemented
}
func (f *fakeClusterCache) AddIndexer(indexName string, indexer mgmtv3.ClusterIndexer) {}
func (f *fakeClusterCache) GetByIndex(indexName, key string) ([]*mgmt.Cluster, error) {
	return nil, errNotImplemented
}
