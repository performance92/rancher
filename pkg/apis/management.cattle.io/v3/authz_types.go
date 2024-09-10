package v3

import (
	"strings"

	"github.com/rancher/norman/condition"
	"github.com/rancher/norman/types"
	v1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1beta1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	NamespaceBackedResource                   condition.Cond = "BackingNamespaceCreated"
	CreatorMadeOwner                          condition.Cond = "CreatorMadeOwner"
	DefaultNetworkPolicyCreated               condition.Cond = "DefaultNetworkPolicyCreated"
	ProjectConditionDefaultNamespacesAssigned condition.Cond = "DefaultNamespacesAssigned"
	ProjectConditionInitialRolesPopulated     condition.Cond = "InitialRolesPopulated"
	ProjectConditionMonitoringEnabled         condition.Cond = "MonitoringEnabled"
	ProjectConditionMetricExpressionDeployed  condition.Cond = "MetricExpressionDeployed"
	ProjectConditionSystemNamespacesAssigned  condition.Cond = "SystemNamespacesAssigned"
)

// +genclient
// +kubebuilder:skipversion
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type Project struct {
	types.Namespaced

	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ProjectSpec   `json:"spec,omitempty"`
	Status ProjectStatus `json:"status"`
}

func (p *Project) ObjClusterName() string {
	return p.Spec.ObjClusterName()
}

type ProjectStatus struct {
	Conditions                    []ProjectCondition `json:"conditions"`
	PodSecurityPolicyTemplateName string             `json:"podSecurityPolicyTemplateId"`
	MonitoringStatus              *MonitoringStatus  `json:"monitoringStatus,omitempty" norman:"nocreate,noupdate"`
}

type ProjectCondition struct {
	// Type of project condition.
	Type string `json:"type"`
	// Status of the condition, one of True, False, Unknown.
	Status v1.ConditionStatus `json:"status"`
	// The last time this condition was updated.
	LastUpdateTime string `json:"lastUpdateTime,omitempty"`
	// Last time the condition transitioned from one status to another.
	LastTransitionTime string `json:"lastTransitionTime,omitempty"`
	// The reason for the condition's last transition.
	Reason string `json:"reason,omitempty"`
	// Human-readable message indicating details about last transition
	Message string `json:"message,omitempty"`
}

type ProjectSpec struct {
	DisplayName                   string                  `json:"displayName,omitempty" norman:"required"`
	Description                   string                  `json:"description"`
	ClusterName                   string                  `json:"clusterName,omitempty" norman:"required,type=reference[cluster]"`
	ResourceQuota                 *ProjectResourceQuota   `json:"resourceQuota,omitempty"`
	NamespaceDefaultResourceQuota *NamespaceResourceQuota `json:"namespaceDefaultResourceQuota,omitempty"`
	ContainerDefaultResourceLimit *ContainerResourceLimit `json:"containerDefaultResourceLimit,omitempty"`
	EnableProjectMonitoring       bool                    `json:"enableProjectMonitoring" norman:"default=false"`
}

func (p *ProjectSpec) ObjClusterName() string {
	return p.ClusterName
}

// +genclient
// +kubebuilder:skipversion
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type GlobalRole struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	DisplayName    string              `json:"displayName,omitempty" norman:"required"`
	Description    string              `json:"description"`
	Rules          []rbacv1.PolicyRule `json:"rules,omitempty"`
	NewUserDefault bool                `json:"newUserDefault,omitempty" norman:"required"`
	Builtin        bool                `json:"builtin" norman:"nocreate,noupdate"`
}

// +genclient
// +kubebuilder:skipversion
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type GlobalRoleBinding struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	UserName           string `json:"userName,omitempty" norman:"noupdate,type=reference[user]"`
	GroupPrincipalName string `json:"groupPrincipalName,omitempty" norman:"noupdate,type=reference[principal]"`
	GlobalRoleName     string `json:"globalRoleName,omitempty" norman:"required,noupdate,type=reference[globalRole]"`
}

// +genclient
// +genclient:nonNamespaced
// +kubebuilder:resource:scope=Cluster
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RoleTemplate holds configuration for a template that is used to create kubernetes Roles and ClusterRoles
// (in the rbac.authorization.k8s.io group) for a cluster or project.
type RoleTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// DisplayName is the human-readable name displayed in the UI for this resource.
	DisplayName string `json:"displayName,omitempty" norman:"required"`

	// Description holds text that describes the resource.
	// +optional
	Description string `json:"description"`

	// Rules hold all the PolicyRules for this RoleTemplate.
	// +optional
	Rules []rbacv1.PolicyRule `json:"rules,omitempty"`

	// Builtin if true specifies that this RoleTemplate was created by Rancher and is immutable.
	// Default to false.
	// +optional
	Builtin bool `json:"builtin" norman:"nocreate,noupdate"`

	// External if true specifies that rules for this RoleTemplate should be gathered from a ClusterRole with the matching name.
	// If set to true the Rules on the template will not be evaluated.
	// External's value is only evaluated if the RoleTemplate's context is set to "cluster"
	// Default to false.
	// +optional
	External bool `json:"external"`

	// ExternalRules hold the external PolicyRules that will be used for authorization.
	// This field is required when External=true and no underlying ClusterRole exists in the local cluster.
	// This field is just used when the feature flag 'external-rules' is on.
	// +optional
	ExternalRules []rbacv1.PolicyRule `json:"externalRules,omitempty"`

	// Hidden if true informs the Rancher UI not to display this RoleTemplate.
	// Default to false.
	// +optional
	Hidden bool `json:"hidden"`

	// Locked if true, new bindings will not be able to use this RoleTemplate.
	// Default to false.
	// +optional
	Locked bool `json:"locked,omitempty" norman:"type=boolean"`

	// ClusterCreatorDefault if true, a binding with this RoleTemplate will be created for a users when they create a new cluster.
	// ClusterCreatorDefault is only evaluated if the context of the RoleTemplate is set to cluster.
	// Default to false.
	// +optional
	ClusterCreatorDefault bool `json:"clusterCreatorDefault,omitempty" norman:"required"`

	// ProjectCreatorDefault if true, a binding with this RoleTemplate will be created for a user when they create a new project.
	// ProjectCreatorDefault is only evaluated if the context of the RoleTemplate is set to project.
	// Default to false.
	// +optional
	ProjectCreatorDefault bool `json:"projectCreatorDefault,omitempty" norman:"required"`

	// Context describes if the roleTemplate applies to clusters or projects.
	// Valid values are "project", "cluster" or "".
	// +kubebuilder:validation:Enum={"project","cluster",""}
	Context string `json:"context,omitempty" norman:"type=string,options=project|cluster"`

	// RoleTemplateNames list of RoleTemplate names that this RoleTemplate will inherit.
	// This RoleTemplate will grant all rules defined in an inherited RoleTemplate.
	// Inherited RoleTemplates must already exist.
	// +optional
	RoleTemplateNames []string `json:"roleTemplateNames,omitempty" norman:"type=array[reference[roleTemplate]]"`

	// Administrative if false, and context is set to cluster this RoleTemplate will not grant access to "CatalogTemplates" and "CatalogTemplateVersions" for any project in the cluster.
	// Default is false.
	// +optional
	Administrative bool `json:"administrative,omitempty"`
}

// +genclient
// +kubebuilder:skipversion
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type PodSecurityPolicyTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Description string                         `json:"description"`
	Spec        policyv1.PodSecurityPolicySpec `json:"spec,omitempty"`
}

// +genclient
// +kubebuilder:skipversion
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type PodSecurityPolicyTemplateProjectBinding struct {
	types.Namespaced
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	PodSecurityPolicyTemplateName string `json:"podSecurityPolicyTemplateId" norman:"required,type=reference[podSecurityPolicyTemplate]"`
	TargetProjectName             string `json:"targetProjectId" norman:"required,type=reference[project]"`
}

// +genclient
// +kubebuilder:skipversion
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ProjectRoleTemplateBinding is the object representing membership of a subject in a project with permissions
// specified by a given role template.
type ProjectRoleTemplateBinding struct {
	types.Namespaced  `json:",inline"`
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// UserName is the name of the user subject added to the project. Immutable.
	// +optional
	UserName string `json:"userName,omitempty" norman:"noupdate,type=reference[user]"`

	// UserPrincipalName is the name of the user principal subject added to the project. Immutable.
	// +optional
	UserPrincipalName string `json:"userPrincipalName,omitempty" norman:"noupdate,type=reference[principal]"`

	// GroupName is the name of the group subject added to the project. Immutable.
	// +optional
	GroupName string `json:"groupName,omitempty" norman:"noupdate,type=reference[group]"`

	// GroupPrincipalName is the name of the group principal subject added to the project. Immutable.
	// +optional
	GroupPrincipalName string `json:"groupPrincipalName,omitempty" norman:"noupdate,type=reference[principal]"`

	// ProjectName is the name of the project to which a subject is added. Immutable.
	// +kubebuilder:validation:Required
	ProjectName string `json:"projectName,omitempty" norman:"required,noupdate,type=reference[project]"`

	// RoleTemplateName is the name of the role template that defines permissions to perform actions on resources in the project. Immutable.
	// +kubebuilder:validation:Required
	RoleTemplateName string `json:"roleTemplateName,omitempty" norman:"required,noupdate,type=reference[roleTemplate]"`

	// ServiceAccount is the name of the service account bound as a subject. Immutable.
	// +optional
	ServiceAccount string `json:"serviceAccount,omitempty" norman:"nocreate,noupdate"`
}

func (p *ProjectRoleTemplateBinding) ObjClusterName() string {
	if parts := strings.SplitN(p.ProjectName, ":", 2); len(parts) == 2 {
		return parts[0]
	}
	return ""
}

// +genclient
// +kubebuilder:skipversion
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type ClusterRoleTemplateBinding struct {
	types.Namespaced
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	UserName           string `json:"userName,omitempty" norman:"noupdate,type=reference[user]"`
	UserPrincipalName  string `json:"userPrincipalName,omitempty" norman:"noupdate,type=reference[principal]"`
	GroupName          string `json:"groupName,omitempty" norman:"noupdate,type=reference[group]"`
	GroupPrincipalName string `json:"groupPrincipalName,omitempty" norman:"noupdate,type=reference[principal]"`
	ClusterName        string `json:"clusterName,omitempty" norman:"required,noupdate,type=reference[cluster]"`
	RoleTemplateName   string `json:"roleTemplateName,omitempty" norman:"required,noupdate,type=reference[roleTemplate]"`
}

func (c *ClusterRoleTemplateBinding) ObjClusterName() string {
	return c.ClusterName
}

type SetPodSecurityPolicyTemplateInput struct {
	PodSecurityPolicyTemplateName string `json:"podSecurityPolicyTemplateId" norman:"type=reference[podSecurityPolicyTemplate]"`
}
