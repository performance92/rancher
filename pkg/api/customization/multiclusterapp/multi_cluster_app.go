package multiclusterapp

import (
	"encoding/json"
	"fmt"
	"github.com/rancher/norman/httperror"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/pkg/errors"
	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/parse"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	gaccess "github.com/rancher/rancher/pkg/api/customization/globalnamespaceaccess"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	managementschema "github.com/rancher/types/apis/management.cattle.io/v3/schema"
	"github.com/rancher/types/client/management/v3"

	"k8s.io/apimachinery/pkg/api/meta"
)

const (
	addProjectsAction    = "addProjects"
	removeProjectsAction = "removeProjects"
)

func (w Wrapper) Formatter(apiContext *types.APIContext, resource *types.RawResource) {
	resource.AddAction(apiContext, "rollback")
	resource.AddAction(apiContext, addProjectsAction)
	resource.AddAction(apiContext, removeProjectsAction)
	resource.Links["revisions"] = apiContext.URLBuilder.Link("revisions", resource)
}

func (w Wrapper) ActionHandler(actionName string, action *types.Action, apiContext *types.APIContext) error {
	var mcApp client.MultiClusterApp
	if err := access.ByID(apiContext, &managementschema.Version, client.MultiClusterAppType, apiContext.ID, &mcApp); err != nil {
		return err
	}
	switch actionName {
	case "rollback":
		data, err := ioutil.ReadAll(apiContext.Request.Body)
		if err != nil {
			return errors.Wrap(err, "reading request body error")
		}
		input := client.MultiClusterAppRollbackInput{}
		if err = json.Unmarshal(data, &input); err != nil {
			return errors.Wrap(err, "unmarshal input error")
		}
		id := input.RevisionID
		splitID := strings.Split(input.RevisionID, ":")
		if len(splitID) == 2 {
			id = splitID[1]
		}
		revision, err := w.MultiClusterAppRevisionLister.Get(namespace.GlobalNamespace, id)
		if err != nil {
			return err
		}
		obj, err := w.MultiClusterAppLister.Get(namespace.GlobalNamespace, mcApp.Name)
		if err != nil {
			return err
		}
		if obj.Status.RevisionName == revision.Name {
			return nil
		}
		toUpdate := obj.DeepCopy()
		toUpdate.Spec.TemplateVersionName = revision.TemplateVersionName
		toUpdate.Spec.Answers = revision.Answers
		_, err = w.MultiClusterApps.Update(toUpdate)
		return err
	case addProjectsAction:
		return w.addProjects(apiContext)
	case removeProjectsAction:
		return w.removeProjects(apiContext)
	default:
		return fmt.Errorf("bad action for multiclusterapp %v", actionName)
	}
}

func (w Wrapper) addProjects(request *types.APIContext) error {
	mcapp, existingProjects, inputProjects, inputAnswers, targetsToRole, err := w.modifyProjects(request, addProjectsAction)
	if err != nil {
		return err
	}
	var mcappToUpdate *v3.MultiClusterApp
	if len(inputProjects) > 0 {
		for _, p := range inputProjects {
			if existingProjects[p] {
				return httperror.NewAPIError(httperror.InvalidBodyContent, fmt.Sprintf("duplicate projects in targets %s", p))
			}
			existingProjects[p] = true
		}
		mcappToUpdate = mcapp.DeepCopy()
		mcappToUpdate.Spec.TargetToRole = targetsToRole
		for _, name := range inputProjects {
			mcappToUpdate.Spec.Targets = append(mcappToUpdate.Spec.Targets, v3.Target{ProjectName: name})
		}
	}
	if len(inputAnswers) > 0 {
		if mcappToUpdate == nil {
			mcappToUpdate = mcapp.DeepCopy()
		}
		mcappToUpdate.Spec.Answers = append(mcappToUpdate.Spec.Answers, inputAnswers...)
	}
	if mcappToUpdate != nil {
		return w.updateMcApp(mcappToUpdate, request, "addedProjects")
	}
	request.WriteResponse(http.StatusOK, nil)
	return nil
}

func (w Wrapper) removeProjects(request *types.APIContext) error {
	mcapp, _, inputProjects, _, _, err := w.modifyProjects(request, removeProjectsAction)
	if err != nil {
		return err
	}
	mcappToUpdate := mcapp.DeepCopy()
	toRemoveProjects := make(map[string]bool)
	var finalTargets []v3.Target
	for _, p := range inputProjects {
		toRemoveProjects[p] = true
	}
	for _, t := range mcapp.Spec.Targets {
		if !toRemoveProjects[t.ProjectName] {
			finalTargets = append(finalTargets, t)
		}
	}
	mcappToUpdate.Spec.Targets = finalTargets
	return w.updateMcApp(mcappToUpdate, request, "removedProjects")
}

func (w Wrapper) modifyProjects(request *types.APIContext, actionName string) (*v3.MultiClusterApp, map[string]bool, []string, []v3.Answer, map[string][]string, error) {
	targetsToRole := make(map[string][]string)
	split := strings.SplitN(request.ID, ":", 2)
	if len(split) != 2 {
		return nil, map[string]bool{}, []string{}, []v3.Answer{}, targetsToRole, fmt.Errorf("incorrect multi cluster app ID %v", request.ID)
	}
	var inputProjects []string
	var inputAnswers []v3.Answer
	existingProjects := make(map[string]bool)
	mcapp, err := w.MultiClusterAppLister.Get(split[0], split[1])
	if err != nil {
		return nil, existingProjects, inputProjects, inputAnswers, targetsToRole, err
	}
	// ensure that caller is not a readonly member of multiclusterapp, else abort
	callerID := request.Request.Header.Get(gaccess.ImpersonateUserHeader)
	metaAccessor, err := meta.Accessor(mcapp)
	if err != nil {
		return nil, existingProjects, inputProjects, inputAnswers, targetsToRole, err
	}
	creatorID, ok := metaAccessor.GetAnnotations()[creatorIDAnn]
	if !ok {
		return nil, existingProjects, inputProjects, inputAnswers, targetsToRole, fmt.Errorf("multiclusterapp %v has no creatorId annotation", metaAccessor.GetName())
	}
	ma := gaccess.MemberAccess{
		Users:              w.Users,
		PrtbLister:         w.PrtbLister,
		CrtbLister:         w.CrtbLister,
		RoleTemplateLister: w.RoleTemplateLister,
		GrbLister:          w.GrbLister,
		GrLister:           w.GrLister,
	}
	accessType, err := ma.GetAccessTypeOfCaller(callerID, creatorID, mcapp.Name, mcapp.Spec.Members)
	if err != nil {
		return nil, existingProjects, inputProjects, inputAnswers, targetsToRole, err
	}
	if accessType != gaccess.OwnerAccess {
		return nil, existingProjects, inputProjects, inputAnswers, targetsToRole, fmt.Errorf("only owners can modify projects of multiclusterapp")
	}
	var updateMultiClusterAppTargetsInput client.UpdateMultiClusterAppTargetsInput
	actionInput, err := parse.ReadBody(request.Request)
	if err != nil {
		return nil, existingProjects, inputProjects, inputAnswers, targetsToRole, err
	}
	if err = convert.ToObj(actionInput, &updateMultiClusterAppTargetsInput); err != nil {
		return nil, existingProjects, inputProjects, inputAnswers, targetsToRole, err
	}
	inputProjects = updateMultiClusterAppTargetsInput.Projects
	for _, p := range mcapp.Spec.Targets {
		existingProjects[p.ProjectName] = true
	}
	if actionName == addProjectsAction && len(inputProjects) > 0 {
		if len(mcapp.Spec.Roles) > 0 {
			if err = ma.EnsureRoleInTargets(inputProjects, mcapp.Spec.Roles, callerID); err != nil {
				return nil, existingProjects, inputProjects, inputAnswers, targetsToRole, err
			}
		} else {
			// no explicit roles on mcapp; derive this caller's roles in target projects, error out if no roles found
			targetsToRole, err := ma.DeriveRolesInTargets(callerID, inputProjects)
			if err != nil {
				return nil, existingProjects, inputProjects, inputAnswers, targetsToRole, err
			}
		}
	}
	for _, a := range updateMultiClusterAppTargetsInput.Answers {
		inputAnswers = append(inputAnswers, v3.Answer{
			ProjectName: a.ProjectID,
			ClusterName: a.ClusterID,
			Values:      a.Values,
		})
	}
	// check if the input includes answers, and if they are only for the input projects
	if len(inputAnswers) > 0 {
		inputProjectsMap := make(map[string]bool)
		for _, p := range inputProjects {
			if !inputProjectsMap[p] {
				inputProjectsMap[p] = true
			}
		}
		for _, a := range inputAnswers {
			if a.ProjectName == "" {
				return nil, existingProjects, inputProjects, inputAnswers, targetsToRole, fmt.Errorf("can only provide project-scoped answers for new projects through add/remove projects action")
			}
			if !inputProjectsMap[a.ProjectName] {
				return nil, existingProjects, inputProjects, inputAnswers, targetsToRole, fmt.Errorf("the project %v is not among the ones provided in input", a.ProjectName)
			}
		}
	}
	return mcapp, existingProjects, inputProjects, inputAnswers, targetsToRole, nil
}

func (w Wrapper) updateMcApp(mcappToUpdate *v3.MultiClusterApp, request *types.APIContext, message string) error {
	if _, err := w.MultiClusterApps.Update(mcappToUpdate); err != nil {
		return err
	}

	op := map[string]interface{}{
		"message": message,
	}
	request.WriteResponse(http.StatusOK, op)
	return nil
}
