package auth

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/rancher/rancher/pkg/controllers/management/authprovisioningv2"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	typesrbacv1 "github.com/rancher/rancher/pkg/generated/norman/rbac.authorization.k8s.io/v1"
	pkgrbac "github.com/rancher/rancher/pkg/rbac"
	"github.com/rancher/rancher/pkg/user"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/retry"

	"github.com/rancher/rancher/pkg/controllers/status"
	"github.com/rancher/rancher/pkg/controllers/status/crtb"
)

const (
	/* Prior to 2.5, the label "memberhsip-binding-owner" was set on the CRB/RBs for a roleTemplateBinding with the key being the roleTemplateBinding's UID.
	2.5 onwards, instead of the roleTemplateBinding's UID, a combination of its namespace and name will be used in this label.
	CRB/RBs on clusters upgraded from 2.4.x to 2.5 will continue to carry the original label with UID. To ensure permissions are managed properly on upgrade,
	we need to change the label value as well.
	So the older label value, MembershipBindingOwnerLegacy (<=2.4.x) will continue to be "memberhsip-binding-owner" (notice the spelling mistake),
	and the new label, MembershipBindingOwner will be "membership-binding-owner" (a different label value with the right spelling)*/
	MembershipBindingOwnerLegacy = "memberhsip-binding-owner"
	MembershipBindingOwner       = "membership-binding-owner"
	clusterResource              = "clusters"
	membershipBindingOwnerIndex  = "auth.management.cattle.io/membership-binding-owner"
	CrtbInProjectBindingOwner    = "crtb-in-project-binding-owner"
	PrtbInClusterBindingOwner    = "prtb-in-cluster-binding-owner"
	rbByOwnerIndex               = "auth.management.cattle.io/rb-by-owner"
	rbByRoleAndSubjectIndex      = "auth.management.cattle.io/crb-by-role-and-subject"
	ctrbMGMTController           = "mgmt-auth-crtb-controller"
	rtbLabelUpdated              = "auth.management.cattle.io/rtb-label-updated"
	RtbCrbRbLabelsUpdated        = "auth.management.cattle.io/crb-rb-labels-updated"
)

var clusterManagementPlaneResources = map[string]string{
	"clusterscans":                "management.cattle.io",
	"catalogtemplates":            "management.cattle.io",
	"catalogtemplateversions":     "management.cattle.io",
	"clusteralertrules":           "management.cattle.io",
	"clusteralertgroups":          "management.cattle.io",
	"clustercatalogs":             "management.cattle.io",
	"clusterloggings":             "management.cattle.io",
	"clustermonitorgraphs":        "management.cattle.io",
	"clusterregistrationtokens":   "management.cattle.io",
	"clusterroletemplatebindings": "management.cattle.io",
	"etcdbackups":                 "management.cattle.io",
	"nodes":                       "management.cattle.io",
	"nodepools":                   "management.cattle.io",
	"notifiers":                   "management.cattle.io",
	"projects":                    "management.cattle.io",
	"etcdsnapshots":               "rke.cattle.io",
}

// Local interface abstracting the mgmtconv3.ClusterRoleTemplateBindingController down to
// necessities. The testsuite then provides a local mock implementation for itself.
type clusterRoleTemplateBindingController interface {
	UpdateStatus(*v3.ClusterRoleTemplateBinding) (*v3.ClusterRoleTemplateBinding, error)
}

type crtbLifecycle struct {
	mgr           managerInterface
	clusterLister v3.ClusterLister
	userMGR       user.Manager
	userLister    v3.UserLister
	projectLister v3.ProjectLister
	rbLister      typesrbacv1.RoleBindingLister
	rbClient      typesrbacv1.RoleBindingInterface
	crbLister     typesrbacv1.ClusterRoleBindingLister
	crbClient     typesrbacv1.ClusterRoleBindingInterface
	crtbClient    v3.ClusterRoleTemplateBindingInterface
	crtbClientM   clusterRoleTemplateBindingController
}

func (c *crtbLifecycle) Create(obj *v3.ClusterRoleTemplateBinding) (runtime.Object, error) {
	// If only the status has been updated and we have finished updating the status
	// (status.Summary != "InProgress") we don't need to perform a reconcile as nothing has
	// changed.
	if (obj.Status.ObservedGeneration == obj.ObjectMeta.Generation &&
		obj.Status.Summary != status.SummaryInProgress) ||
		status.HasAllOf(obj.Status.Conditions, crtb.LocalSuccesses) {
		return obj, nil
	}
	if err := c.setCRTBAsInProgress(obj); err != nil {
		return obj, err
	}
	if err := c.reconcileSubject(obj); err != nil {
		return obj, err
	}
	if err := c.reconcileBindings(obj); err != nil {
		return obj, err
	}
	err := c.setCRTBAsCompleted(obj)
	return obj, err
}

func (c *crtbLifecycle) Updated(obj *v3.ClusterRoleTemplateBinding) (runtime.Object, error) {
	// ObjectMeta.Generation does not get updated when the Status is updated.
	// If only the status has been updated and we have finished updating the status
	// (status.Summary != "InProgress") we don't need to perform a reconcile as nothing has
	// changed.
	if (obj.Status.ObservedGeneration == obj.ObjectMeta.Generation &&
		obj.Status.Summary != status.SummaryInProgress) ||
		status.HasAllOf(obj.Status.Conditions, crtb.LocalSuccesses) {
		return obj, nil
	}
	if err := c.setCRTBAsInProgress(obj); err != nil {
		return obj, err
	}
	if err := c.reconcileSubject(obj); err != nil {
		return obj, err
	}
	if err := c.reconcileLabels(obj); err != nil {
		return obj, err
	}
	if err := c.reconcileBindings(obj); err != nil {
		return obj, err
	}
	err := c.setCRTBAsCompleted(obj)
	return obj, err
}

func (c *crtbLifecycle) Remove(obj *v3.ClusterRoleTemplateBinding) (runtime.Object, error) {
	if err := c.setCRTBAsTerminating(obj); err != nil {
		return obj, err
	}
	if err := c.reconcileClusterMembershipBindingForDelete(obj); err != nil {
		return nil, err
	}
	if err := c.removeMGMTClusterScopedPrivilegesInProjectNamespace(obj); err != nil {
		return nil, err
	}
	err := c.removeAuthV2Permissions(obj)
	return nil, err
}

func (c *crtbLifecycle) reconcileSubject(binding *v3.ClusterRoleTemplateBinding) error {
	condition := v1.Condition{Type: crtb.SubjectExists}

	if binding.GroupName != "" || binding.GroupPrincipalName != "" ||
		(binding.UserPrincipalName != "" && binding.UserName != "") {
		addCondition(binding, condition, crtb.SubjectExists, binding.UserName, nil)
		return nil
	}

	if binding.UserPrincipalName != "" && binding.UserName == "" {
		displayName := binding.Annotations["auth.cattle.io/principal-display-name"]
		user, err := c.userMGR.EnsureUser(binding.UserPrincipalName, displayName)
		if err != nil {
			addCondition(binding, condition, crtb.FailedToGetSubject, binding.UserPrincipalName, err)
			return err
		}

		binding.UserName = user.Name
		addCondition(binding, condition, crtb.SubjectExists, binding.UserName, nil)
		return nil
	}

	if binding.UserPrincipalName == "" && binding.UserName != "" {
		u, err := c.userLister.Get("", binding.UserName)
		if err != nil {
			addCondition(binding, condition, crtb.FailedToGetSubject, binding.UserName, err)
			return err
		}
		for _, p := range u.PrincipalIDs {
			if strings.HasSuffix(p, binding.UserName) {
				binding.UserPrincipalName = p
				break
			}
		}
		addCondition(binding, condition, crtb.SubjectExists, binding.UserPrincipalName, nil)
		return nil
	}

	err := fmt.Errorf("ClusterRoleTemplateBinding %v has no subject", binding.Name)
	addCondition(binding, condition, crtb.FailedToGetSubject, binding.Name, err)
	return err
}

// When a CRTB is created or updated, translate it into several k8s roles and bindings to actually enforce the RBAC
// Specifically:
// - ensure the subject can see the cluster in the mgmt API
// - if the subject was granted owner permissions for the clsuter, ensure they can create/update/delete the cluster
// - if the subject was granted privileges to mgmt plane resources that are scoped to the cluster, enforce those rules in the cluster's mgmt plane namespace
func (c *crtbLifecycle) reconcileBindings(binding *v3.ClusterRoleTemplateBinding) error {
	condition := v1.Condition{Type: crtb.LocalBindingsExist}

	if binding.UserName == "" && binding.GroupPrincipalName == "" && binding.GroupName == "" {
		addCondition(binding, condition, crtb.LocalBindingsExist, binding.Name, nil)
		return nil
	}

	clusterName := binding.ClusterName
	cluster, err := c.clusterLister.Get("", clusterName)
	if err != nil {
		addCondition(binding, condition, crtb.FailedToGetCluster, binding.Name, err)
		return err
	}
	if cluster == nil {
		err := fmt.Errorf("cannot create binding because cluster %v was not found", clusterName)
		addCondition(binding, condition, crtb.FailedToGetCluster, binding.Name, err)
		return err
	}
	// if roletemplate is not builtin, check if it's inherited/cloned
	isOwnerRole, err := c.mgr.checkReferencedRoles(binding.RoleTemplateName, clusterContext, 0)
	if err != nil {
		addCondition(binding, condition, crtb.BadRoleReferences, binding.Name, err)
		return err
	}
	var clusterRoleName string
	if isOwnerRole {
		clusterRoleName = strings.ToLower(fmt.Sprintf("%v-clusterowner", clusterName))
	} else {
		clusterRoleName = strings.ToLower(fmt.Sprintf("%v-clustermember", clusterName))
	}

	subject, err := pkgrbac.BuildSubjectFromRTB(binding)
	if err != nil {
		addCondition(binding, condition, crtb.FailedToBuildSubject, binding.Name, err)
		return err
	}
	if err := c.mgr.ensureClusterMembershipBinding(clusterRoleName, pkgrbac.GetRTBLabel(binding.ObjectMeta), cluster, isOwnerRole, subject); err != nil {
		addCondition(binding, condition, crtb.FailedToEnsureClusterMembership, binding.Name, err)
		return err
	}

	err = c.mgr.grantManagementPlanePrivileges(binding.RoleTemplateName, clusterManagementPlaneResources, subject, binding)
	if err != nil {
		addCondition(binding, condition, crtb.FailedToGrantManagementPlanePrivileges, binding.Name, err)
		return err
	}

	projects, err := c.projectLister.List(binding.Namespace, labels.Everything())
	if err != nil {
		addCondition(binding, condition, crtb.FailedToGetNamespace, binding.Name, err)
		return err
	}
	for _, p := range projects {
		if p.DeletionTimestamp != nil {
			logrus.Warnf("Project %v is being deleted, not creating membership bindings", p.Name)
			continue
		}
		if err := c.mgr.grantManagementClusterScopedPrivilegesInProjectNamespace(binding.RoleTemplateName, p.Name, projectManagementPlaneResources, subject, binding); err != nil {
			addCondition(binding, condition, crtb.FailedToGrantManagementClusterPrivileges, binding.Name, err)
			return err
		}
	}

	addCondition(binding, condition, crtb.LocalBindingsExist, binding.Name, nil)
	return nil
}

func (c *crtbLifecycle) reconcileClusterMembershipBindingForDelete(binding *v3.ClusterRoleTemplateBinding) error {
	condition := v1.Condition{Type: crtb.ClusterMembershipBindingForDeleteOk}

	err := c.mgr.reconcileClusterMembershipBindingForDelete("", pkgrbac.GetRTBLabel(binding.ObjectMeta))
	if err != nil {
		addCondition(binding, condition, crtb.FailedClusterMembershipBindingForDelete, binding.UserName, err)
	}

	addCondition(binding, condition, crtb.ClusterMembershipBindingForDeleteOk, binding.UserName, nil)
	return err
}

func (c *crtbLifecycle) removeAuthV2Permissions(binding *v3.ClusterRoleTemplateBinding) error {
	condition := v1.Condition{Type: crtb.AuthV2PermissionsOk}

	err := c.mgr.removeAuthV2Permissions(authprovisioningv2.CRTBRoleBindingID, binding)
	if err != nil {
		addCondition(binding, condition, crtb.FailedRemovalOfAuthV2Permissions, binding.UserName, err)
	}

	addCondition(binding, condition, crtb.AuthV2PermissionsOk, binding.UserName, nil)
	return err
}

func (c *crtbLifecycle) removeMGMTClusterScopedPrivilegesInProjectNamespace(binding *v3.ClusterRoleTemplateBinding) error {
	condition := v1.Condition{Type: crtb.LocalCRTBDeleteOk}

	projects, err := c.projectLister.List(binding.Namespace, labels.Everything())
	if err != nil {
		addCondition(binding, condition, crtb.FailedRemovalOfMGMTClusterScopedPrivilegesInProjectNamespace, binding.UserName, err)
		return err
	}
	bindingKey := pkgrbac.GetRTBLabel(binding.ObjectMeta)
	for _, p := range projects {
		set := labels.Set(map[string]string{bindingKey: CrtbInProjectBindingOwner})
		rbs, err := c.rbLister.List(p.Name, set.AsSelector())
		if err != nil {
			addCondition(binding, condition, crtb.FailedRemovalOfMGMTClusterScopedPrivilegesInProjectNamespace, binding.UserName, err)
			return err
		}
		for _, rb := range rbs {
			logrus.Infof("[%v] Deleting rolebinding %v in namespace %v for crtb %v", ctrbMGMTController, rb.Name, p.Name, binding.Name)
			if err := c.rbClient.DeleteNamespaced(p.Name, rb.Name, &v1.DeleteOptions{}); err != nil {
				addCondition(binding, condition, crtb.FailedRemovalOfMGMTClusterScopedPrivilegesInProjectNamespace, binding.UserName, err)
				return err
			}
		}
	}

	addCondition(binding, condition, crtb.LocalCRTBDeleteOk, binding.UserName, nil)
	return nil
}

func (c *crtbLifecycle) reconcileLabels(binding *v3.ClusterRoleTemplateBinding) error {
	condition := v1.Condition{Type: crtb.LocalLabelsSet}

	/* Prior to 2.5, for every CRTB, following CRBs and RBs are created in the management clusters
		1. CRTB.UID is the label key for a CRB, CRTB.UID=memberhsip-binding-owner
	    2. CRTB.UID is label key for the RB, CRTB.UID=crtb-in-project-binding-owner (in the namespace of each project in the cluster that the user has access to)
	Using above labels, list the CRB and RB and update them to add a label with ns+name of CRTB
	*/
	if binding.Labels[RtbCrbRbLabelsUpdated] == "true" {
		addCondition(binding, condition, crtb.LocalLabelsSet, binding.Name, nil)
		return nil
	}

	var returnErr error
	requirements, err := getLabelRequirements(binding.ObjectMeta)
	if err != nil {
		addCondition(binding, condition, crtb.LocalFailedToGetLabelRequirements, binding.Name, err)
		return err
	}

	set := labels.Set(map[string]string{string(binding.UID): MembershipBindingOwnerLegacy})
	crbs, err := c.crbLister.List(v1.NamespaceAll, set.AsSelector().Add(requirements...))
	if err != nil {
		addCondition(binding, condition, crtb.LocalFailedToGetClusterRoleBindings, binding.Name, err)
		return err
	}
	bindingKey := pkgrbac.GetRTBLabel(binding.ObjectMeta)
	for _, crb := range crbs {
		retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			crbToUpdate, updateErr := c.crbClient.Get(crb.Name, v1.GetOptions{})
			if updateErr != nil {
				return updateErr
			}
			if crbToUpdate.Labels == nil {
				crbToUpdate.Labels = make(map[string]string)
			}
			crbToUpdate.Labels[bindingKey] = MembershipBindingOwner
			crbToUpdate.Labels[rtbLabelUpdated] = "true"
			_, err := c.crbClient.Update(crbToUpdate)
			return err
		})
		if retryErr != nil {
			addCondition(binding, condition, crtb.LocalFailedToUpdateClusterRoleBindings, binding.Name, retryErr)
		}
		returnErr = errors.Join(returnErr, retryErr)
	}

	set = map[string]string{string(binding.UID): CrtbInProjectBindingOwner}
	rbs, err := c.rbLister.List(v1.NamespaceAll, set.AsSelector().Add(requirements...))
	if err != nil {
		addCondition(binding, condition, crtb.FailedToGetRoleBindings, binding.Name, err)
		return err
	}

	for _, rb := range rbs {
		retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			rbToUpdate, updateErr := c.rbClient.GetNamespaced(rb.Namespace, rb.Name, v1.GetOptions{})
			if updateErr != nil {
				return updateErr
			}
			if rbToUpdate.Labels == nil {
				rbToUpdate.Labels = make(map[string]string)
			}
			rbToUpdate.Labels[bindingKey] = CrtbInProjectBindingOwner
			rbToUpdate.Labels[rtbLabelUpdated] = "true"
			_, err := c.rbClient.Update(rbToUpdate)
			return err
		})
		returnErr = errors.Join(returnErr, retryErr)
	}
	if returnErr != nil {
		addCondition(binding, condition, crtb.FailedToUpdateRoleBindings, binding.Name, returnErr)
		return returnErr
	}

	retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		crtbToUpdate, updateErr := c.crtbClient.GetNamespaced(binding.Namespace, binding.Name, v1.GetOptions{})
		if updateErr != nil {
			return updateErr
		}
		if crtbToUpdate.Labels == nil {
			crtbToUpdate.Labels = make(map[string]string)
		}
		crtbToUpdate.Labels[RtbCrbRbLabelsUpdated] = "true"
		_, err := c.crtbClient.Update(crtbToUpdate)
		return err
	})
	if retryErr != nil {
		addCondition(binding, condition, crtb.LocalFailedToUpdateCRTBLabels, binding.Name, retryErr)
		return retryErr
	}

	addCondition(binding, condition, crtb.LocalLabelsSet, binding.Name, nil)
	return nil
}

// Status field management, condition management

func (c *crtbLifecycle) setCRTBAsInProgress(binding *v3.ClusterRoleTemplateBinding) error {
	// Keep information managed by the remote controller.
	// Wipe only information managed here
	binding.Status.Conditions = status.RemoveConditions(binding.Status.Conditions, crtb.LocalConditions)

	binding.Status.Summary = status.SummaryInProgress
	binding.Status.LastUpdateTime = time.Now().String()
	updatedCRTB, err := c.crtbClientM.UpdateStatus(binding)
	if err != nil {
		return err
	}
	// For future updates, we want the latest version of our CRTB
	*binding = *updatedCRTB
	return nil
}

func (c *crtbLifecycle) setCRTBAsCompleted(binding *v3.ClusterRoleTemplateBinding) error {
	// set summary based on error conditions
	failed := false
	for _, c := range binding.Status.Conditions {
		if c.Status != v1.ConditionTrue {
			binding.Status.Summary = status.SummaryError
			failed = true
			break
		}
	}

	// no error conditions. check for all (local and remote!) success conditions
	// note: keep the status as in progress if only partial sucess was found
	if !failed && status.HasAllOf(binding.Status.Conditions, crtb.Successes) {
		binding.Status.Summary = status.SummaryCompleted
	}

	binding.Status.LastUpdateTime = time.Now().String()
	binding.Status.ObservedGeneration = binding.ObjectMeta.Generation
	updatedCRTB, err := c.crtbClientM.UpdateStatus(binding)
	if err != nil {
		return err
	}
	// For future updates, we want the latest version of our CRTB
	*binding = *updatedCRTB
	return nil
}

func (c *crtbLifecycle) setCRTBAsTerminating(binding *v3.ClusterRoleTemplateBinding) error {
	binding.Status.Conditions = []v1.Condition{}
	binding.Status.Summary = status.SummaryTerminating
	binding.Status.LastUpdateTime = time.Now().String()
	_, err := c.crtbClientM.UpdateStatus(binding)
	return err
}

func addCondition(binding *v3.ClusterRoleTemplateBinding, condition v1.Condition, reason, name string, err error) {
	if err != nil {
		condition.Status = v1.ConditionFalse
		condition.Message = fmt.Sprintf("%s not created: %v", name, err)
	} else {
		condition.Status = v1.ConditionTrue
		condition.Message = fmt.Sprintf("%s created", name)
	}
	condition.Reason = reason
	condition.LastTransitionTime = v1.Time{Time: time.Now()}
	binding.Status.Conditions = append(binding.Status.Conditions, condition)
}
