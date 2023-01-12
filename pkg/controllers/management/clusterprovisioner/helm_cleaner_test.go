package clusterprovisioner

import (
	"github.com/helm/helm-mapkubeapis/pkg/mapping"
	"github.com/stretchr/testify/assert"
	"testing"
)

var (
	invalidApiMappings = &mapping.Metadata{
		Mappings: []*mapping.Mapping{
			{
				DeprecatedAPI:    "apiVersion: policy/v1beta1\nkind: PodSecurityPolicy\n",
				RemovedInVersion: "some-invalid-version",
			},
		},
	}
)

func TestReplaceManifestDataWithValidMappings(t *testing.T) {
	tests := []struct {
		name              string
		testManifest      string
		kubernetesVersion string
		resultManifest    string
		replaced          bool
		errorOccurs       bool
		errorMessage      string
	}{
		{
			name: "PSPs get correctly removed on k8s v1.25 when PSP is the first object",
			testManifest: `---
apiVersion: policy/v1beta1
kind: PodSecurityPolicy
metadata:
  name: test
spec:
  allowPrivilegeEscalation: false
---
apiVersion: apps/v1
kind: Deployment
metadata:
  creationTimestamp: null
  labels:
    app: test-deploy
  name: test-deploy
spec:
  replicas: 1
  selector:
    matchLabels:
      app: test-deploy
  strategy: {}
  template:
    metadata:
      creationTimestamp: null
      labels:
        app: test-deploy
    spec:
      containers:
      - image: registry.k8s.io/pause
        name: pause
        resources: {}
`,
			kubernetesVersion: "v1.25",
			resultManifest: `---
apiVersion: apps/v1
kind: Deployment
metadata:
  creationTimestamp: null
  labels:
    app: test-deploy
  name: test-deploy
spec:
  replicas: 1
  selector:
    matchLabels:
      app: test-deploy
  strategy: {}
  template:
    metadata:
      creationTimestamp: null
      labels:
        app: test-deploy
    spec:
      containers:
      - image: registry.k8s.io/pause
        name: pause
        resources: {}`,
			replaced: true,
		},
		{
			name: "PSPs get correctly removed on k8s v1.25 when PSP is the last object",
			testManifest: `---
apiVersion: apps/v1
kind: Deployment
metadata:
  creationTimestamp: null
  labels:
    app: test-deploy
  name: test-deploy
spec:
  replicas: 1
  selector:
    matchLabels:
      app: test-deploy
  strategy: {}
  template:
    metadata:
      creationTimestamp: null
      labels:
        app: test-deploy
    spec:
      containers:
      - image: registry.k8s.io/pause
        name: pause
        resources: {}
---
apiVersion: policy/v1beta1
kind: PodSecurityPolicy
metadata:
  name: test
spec:
  allowPrivilegeEscalation: false
`,
			kubernetesVersion: "v1.25",
			resultManifest: `---
apiVersion: apps/v1
kind: Deployment
metadata:
  creationTimestamp: null
  labels:
    app: test-deploy
  name: test-deploy
spec:
  replicas: 1
  selector:
    matchLabels:
      app: test-deploy
  strategy: {}
  template:
    metadata:
      creationTimestamp: null
      labels:
        app: test-deploy
    spec:
      containers:
      - image: registry.k8s.io/pause
        name: pause
        resources: {}`,
			replaced: true,
		},
		{
			name: "PSPs get correctly removed on k8s v1.25 when PSP is in the middle of the manifest",
			testManifest: `---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: a-test-sa
---
apiVersion: policy/v1beta1
kind: PodSecurityPolicy
metadata:
  name: test
spec:
  allowPrivilegeEscalation: false
---
apiVersion: apps/v1
kind: Deployment
metadata:
  creationTimestamp: null
  labels:
    app: test-deploy
  name: test-deploy
spec:
  replicas: 1
  selector:
    matchLabels:
      app: test-deploy
  strategy: {}
  template:
    metadata:
      creationTimestamp: null
      labels:
        app: test-deploy
    spec:
      containers:
      - image: registry.k8s.io/pause
        name: pause
        resources: {}
`,
			kubernetesVersion: "v1.25",
			resultManifest: `---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: a-test-sa
---
apiVersion: apps/v1
kind: Deployment
metadata:
  creationTimestamp: null
  labels:
    app: test-deploy
  name: test-deploy
spec:
  replicas: 1
  selector:
    matchLabels:
      app: test-deploy
  strategy: {}
  template:
    metadata:
      creationTimestamp: null
      labels:
        app: test-deploy
    spec:
      containers:
      - image: registry.k8s.io/pause
        name: pause
        resources: {}`,
			replaced: true,
		},
		{
			name: "PSPs get correctly removed on k8s v1.25 when the first three-dash is missing and PSP is the first object",
			testManifest: `apiVersion: policy/v1beta1
kind: PodSecurityPolicy
metadata:
  name: test
spec:
  allowPrivilegeEscalation: false
---
apiVersion: apps/v1
kind: Deployment
metadata:
  creationTimestamp: null
  labels:
    app: test-deploy
  name: test-deploy
spec:
  replicas: 1
  selector:
    matchLabels:
      app: test-deploy
  strategy: {}
  template:
    metadata:
      creationTimestamp: null
      labels:
        app: test-deploy
    spec:
      containers:
      - image: registry.k8s.io/pause
        name: pause
        resources: {}
`,
			kubernetesVersion: "v1.25",
			resultManifest: `---
apiVersion: apps/v1
kind: Deployment
metadata:
  creationTimestamp: null
  labels:
    app: test-deploy
  name: test-deploy
spec:
  replicas: 1
  selector:
    matchLabels:
      app: test-deploy
  strategy: {}
  template:
    metadata:
      creationTimestamp: null
      labels:
        app: test-deploy
    spec:
      containers:
      - image: registry.k8s.io/pause
        name: pause
        resources: {}`,
			replaced: true,
		},
		{
			name: "PSPs get removed correctly when there is more than one PSP object",
			testManifest: `apiVersion: policy/v1beta1
kind: PodSecurityPolicy
metadata:
  name: test
spec:
  allowPrivilegeEscalation: false
---
apiVersion: apps/v1
kind: Deployment
metadata:
  creationTimestamp: null
  labels:
    app: test-deploy
  name: test-deploy
spec:
  replicas: 1
  selector:
    matchLabels:
      app: test-deploy
  strategy: {}
  template:
    metadata:
      creationTimestamp: null
      labels:
        app: test-deploy
    spec:
      containers:
      - image: registry.k8s.io/pause
        name: pause
        resources: {}
---
apiVersion: policy/v1beta1
kind: PodSecurityPolicy
metadata:
  name: second-test
spec:
  allowPrivilegeEscalation: false`,
			kubernetesVersion: "v1.25",
			resultManifest: `---
apiVersion: apps/v1
kind: Deployment
metadata:
  creationTimestamp: null
  labels:
    app: test-deploy
  name: test-deploy
spec:
  replicas: 1
  selector:
    matchLabels:
      app: test-deploy
  strategy: {}
  template:
    metadata:
      creationTimestamp: null
      labels:
        app: test-deploy
    spec:
      containers:
      - image: registry.k8s.io/pause
        name: pause
        resources: {}`,
			replaced: true,
		},
		{
			name: "PSP does not get removed when Kubernetes version < 1.25",
			testManifest: `apiVersion: policy/v1beta1
kind: PodSecurityPolicy
metadata:
  name: test
spec:
  allowPrivilegeEscalation: false
---
apiVersion: apps/v1
kind: Deployment
metadata:
  creationTimestamp: null
  labels:
    app: test-deploy
  name: test-deploy
spec:
  replicas: 1
  selector:
    matchLabels:
      app: test-deploy
  strategy: {}
  template:
    metadata:
      creationTimestamp: null
      labels:
        app: test-deploy
    spec:
      containers:
      - image: registry.k8s.io/pause
        name: pause
        resources: {}`,
			kubernetesVersion: "v1.24",
			resultManifest: `apiVersion: policy/v1beta1
kind: PodSecurityPolicy
metadata:
  name: test
spec:
  allowPrivilegeEscalation: false
---
apiVersion: apps/v1
kind: Deployment
metadata:
  creationTimestamp: null
  labels:
    app: test-deploy
  name: test-deploy
spec:
  replicas: 1
  selector:
    matchLabels:
      app: test-deploy
  strategy: {}
  template:
    metadata:
      creationTimestamp: null
      labels:
        app: test-deploy
    spec:
      containers:
      - image: registry.k8s.io/pause
        name: pause
        resources: {}`,
			replaced: false,
		},
		{
			name: "Manifest is not edited when no outdated resources are found",
			testManifest: `---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: test-sa
---
apiVersion: apps/v1
kind: Deployment
metadata:
  creationTimestamp: null
  labels:
    app: test-deploy
  name: test-deploy
spec:
  replicas: 1
  selector:
    matchLabels:
      app: test-deploy
  strategy: {}
  template:
    metadata:
      creationTimestamp: null
      labels:
        app: test-deploy
    spec:
      containers:
      - image: registry.k8s.io/pause
        name: pause
        resources: {}`,
			kubernetesVersion: "v1.24",
			resultManifest: `---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: test-sa
---
apiVersion: apps/v1
kind: Deployment
metadata:
  creationTimestamp: null
  labels:
    app: test-deploy
  name: test-deploy
spec:
  replicas: 1
  selector:
    matchLabels:
      app: test-deploy
  strategy: {}
  template:
    metadata:
      creationTimestamp: null
      labels:
        app: test-deploy
    spec:
      containers:
      - image: registry.k8s.io/pause
        name: pause
        resources: {}`,
			replaced: false,
		},
		{
			name: "Should fail when the Kubernetes version passed is invalid",
			testManifest: `---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: test-sa`,
			kubernetesVersion: "invalid-kube-version",
			resultManifest:    "",
			replaced:          false,
			errorOccurs:       true,
			errorMessage:      "Invalid format for Kubernetes semantic version",
		},
		{
			name: "PodDisruptionBudget does not get replaced when Kubernetes version < 1.25",
			testManifest: `apiVersion: policy/v1beta1
kind: PodDisruptionBudget
metadata:
  name: test-pdb
spec:
  maxUnavailable: 2
---
apiVersion: apps/v1
kind: Deployment
metadata:
  creationTimestamp: null
  labels:
    app: test-deploy
  name: test-deploy
spec:
  replicas: 1
  selector:
    matchLabels:
      app: test-deploy
  strategy: {}
  template:
    metadata:
      creationTimestamp: null
      labels:
        app: test-deploy
    spec:
      containers:
      - image: registry.k8s.io/pause
        name: pause
        resources: {}`,
			kubernetesVersion: "v1.24",
			resultManifest: `apiVersion: policy/v1beta1
kind: PodDisruptionBudget
metadata:
  name: test-pdb
spec:
  maxUnavailable: 2
---
apiVersion: apps/v1
kind: Deployment
metadata:
  creationTimestamp: null
  labels:
    app: test-deploy
  name: test-deploy
spec:
  replicas: 1
  selector:
    matchLabels:
      app: test-deploy
  strategy: {}
  template:
    metadata:
      creationTimestamp: null
      labels:
        app: test-deploy
    spec:
      containers:
      - image: registry.k8s.io/pause
        name: pause
        resources: {}`,
			replaced: false,
		},
		{
			name: "PodDisruptionBudget is replaced by its successor when Kubernetes version >= 1.25",
			testManifest: `apiVersion: policy/v1beta1
kind: PodDisruptionBudget
metadata:
  name: test-pdb
spec:
  maxUnavailable: 2
---
apiVersion: apps/v1
kind: Deployment
metadata:
  creationTimestamp: null
  labels:
    app: test-deploy
  name: test-deploy
spec:
  replicas: 1
  selector:
    matchLabels:
      app: test-deploy
  strategy: {}
  template:
    metadata:
      creationTimestamp: null
      labels:
        app: test-deploy
    spec:
      containers:
      - image: registry.k8s.io/pause
        name: pause
        resources: {}`,
			kubernetesVersion: "v1.25",
			resultManifest: `apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: test-pdb
spec:
  maxUnavailable: 2
---
apiVersion: apps/v1
kind: Deployment
metadata:
  creationTimestamp: null
  labels:
    app: test-deploy
  name: test-deploy
spec:
  replicas: 1
  selector:
    matchLabels:
      app: test-deploy
  strategy: {}
  template:
    metadata:
      creationTimestamp: null
      labels:
        app: test-deploy
    spec:
      containers:
      - image: registry.k8s.io/pause
        name: pause
        resources: {}`,
			replaced: true,
		},
		{
			name: "CronJob does not get replaced when Kubernetes version < 1.25",
			testManifest: `apiVersion: batch/v1beta1
kind: CronJob
metadata:
  creationTimestamp: null
  name: test-job
spec:
  jobTemplate:
    metadata:
      creationTimestamp: null
      name: test-job
    spec:
      template:
        metadata:
          creationTimestamp: null
        spec:
          containers:
          - image: test
            name: test-job
            resources: {}
          restartPolicy: OnFailure
  schedule: '*/5'
status: {}`,
			kubernetesVersion: "v1.24",
			resultManifest: `apiVersion: batch/v1beta1
kind: CronJob
metadata:
  creationTimestamp: null
  name: test-job
spec:
  jobTemplate:
    metadata:
      creationTimestamp: null
      name: test-job
    spec:
      template:
        metadata:
          creationTimestamp: null
        spec:
          containers:
          - image: test
            name: test-job
            resources: {}
          restartPolicy: OnFailure
  schedule: '*/5'
status: {}`,
			replaced: false,
		},
		{
			name: "CronJob is replaced by its successor when Kubernetes version >= 1.25",
			testManifest: `apiVersion: batch/v1beta1
kind: CronJob
metadata:
  creationTimestamp: null
  name: test-job
spec:
  jobTemplate:
    metadata:
      creationTimestamp: null
      name: test-job
    spec:
      template:
        metadata:
          creationTimestamp: null
        spec:
          containers:
          - image: test
            name: test-job
            resources: {}
          restartPolicy: OnFailure
  schedule: '*/5'
status: {}`,
			kubernetesVersion: "v1.25",
			resultManifest: `apiVersion: batch/v1
kind: CronJob
metadata:
  creationTimestamp: null
  name: test-job
spec:
  jobTemplate:
    metadata:
      creationTimestamp: null
      name: test-job
    spec:
      template:
        metadata:
          creationTimestamp: null
        spec:
          containers:
          - image: test
            name: test-job
            resources: {}
          restartPolicy: OnFailure
  schedule: '*/5'
status: {}`,
			replaced: true,
		},
		{
			name: "HorizontalPodAutoscaler does not get replaced when Kubernetes version < 1.25",
			testManifest: `---
apiVersion: apps/v1
kind: Deployment
metadata:
  creationTimestamp: null
  labels:
    app: test-deploy
  name: test-deploy
spec:
  replicas: 1
  selector:
    matchLabels:
      app: test-deploy
  strategy: {}
  template:
    metadata:
      creationTimestamp: null
      labels:
        app: test-deploy
    spec:
      containers:
      - image: registry.k8s.io/pause
        name: pause
        resources: {}
---
apiVersion: autoscaling/v2beta1
kind: HorizontalPodAutoscaler
metadata:
  name: test-hpa
spec:
  maxReplicas: 4
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: test-deploy`,
			kubernetesVersion: "v1.24",
			resultManifest: `---
apiVersion: apps/v1
kind: Deployment
metadata:
  creationTimestamp: null
  labels:
    app: test-deploy
  name: test-deploy
spec:
  replicas: 1
  selector:
    matchLabels:
      app: test-deploy
  strategy: {}
  template:
    metadata:
      creationTimestamp: null
      labels:
        app: test-deploy
    spec:
      containers:
      - image: registry.k8s.io/pause
        name: pause
        resources: {}
---
apiVersion: autoscaling/v2beta1
kind: HorizontalPodAutoscaler
metadata:
  name: test-hpa
spec:
  maxReplicas: 4
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: test-deploy`,
			replaced: false,
		},
		{
			name: "HorizontalPodAutoscaler is replaced by its successor when Kubernetes version >= 1.25",
			testManifest: `---
apiVersion: apps/v1
kind: Deployment
metadata:
  creationTimestamp: null
  labels:
    app: test-deploy
  name: test-deploy
spec:
  replicas: 1
  selector:
    matchLabels:
      app: test-deploy
  strategy: {}
  template:
    metadata:
      creationTimestamp: null
      labels:
        app: test-deploy
    spec:
      containers:
      - image: registry.k8s.io/pause
        name: pause
        resources: {}
---
apiVersion: autoscaling/v2beta1
kind: HorizontalPodAutoscaler
metadata:
  name: test-hpa
spec:
  maxReplicas: 4
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: test-deploy`,
			kubernetesVersion: "v1.25",
			resultManifest: `---
apiVersion: apps/v1
kind: Deployment
metadata:
  creationTimestamp: null
  labels:
    app: test-deploy
  name: test-deploy
spec:
  replicas: 1
  selector:
    matchLabels:
      app: test-deploy
  strategy: {}
  template:
    metadata:
      creationTimestamp: null
      labels:
        app: test-deploy
    spec:
      containers:
      - image: registry.k8s.io/pause
        name: pause
        resources: {}
---
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: test-hpa
spec:
  maxReplicas: 4
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: test-deploy`,
			replaced: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			replaced, modifiedManifest, err := ReplaceManifestData(apiMappings, tt.testManifest, tt.kubernetesVersion)

			if tt.errorOccurs {
				assert.NotNil(t, err)
				assert.ErrorContains(t, err, tt.errorMessage)
			} else {
				assert.Nil(t, err)
				assert.Equal(t, tt.replaced, replaced)
				assert.Equal(t, tt.resultManifest, modifiedManifest)
			}
		})
	}
}

func TestReplaceManifestDataWithInvalidMappings(t *testing.T) {
	t.Run("Should fail when mappings have an invalid Kubernetes semantic version", func(t *testing.T) {
		manifest := `---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: test-sa`

		replaced, modifiedManifest, err := ReplaceManifestData(invalidApiMappings, manifest, "v1.25")

		assert.False(t, replaced)
		assert.Empty(t, modifiedManifest)
		assert.ErrorContains(t, err, "Failed to get the deprecated or removed Kubernetes version for API")
	})
}
