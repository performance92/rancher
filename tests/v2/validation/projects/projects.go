package projects

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	"github.com/rancher/shepherd/extensions/defaults/timeouts"
	projectsApi "github.com/rancher/shepherd/extensions/kubeapi/projects"
	rbacApi "github.com/rancher/shepherd/extensions/kubeapi/rbac"
	"github.com/rancher/shepherd/extensions/kubeconfig"
	namegen "github.com/rancher/shepherd/pkg/namegenerator"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kwait "k8s.io/apimachinery/pkg/util/wait"
)

const (
	dummyFinalizer   = "dummy"
	timeFormat       = "2006/01/02 15:04:05"
	roleProjectOwner = "project-owner"
	roleOwner        = "cluster-owner"
)

var project = v3.Project{
	ObjectMeta: metav1.ObjectMeta{
		Name:       "",
		Namespace:  "",
		Finalizers: []string{},
	},
	Spec: v3.ProjectSpec{
		ClusterName: "",
	},
}

var prtb = v3.ProjectRoleTemplateBinding{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "",
		Namespace: "",
	},
	ProjectName:       "",
	RoleTemplateName:  "",
	UserPrincipalName: "",
}

func createProject(client *rancher.Client, clusterName string) (*v3.Project, error) {
	project.Name = namegen.AppendRandomString("testproject")
	project.Namespace = clusterName
	project.Spec.ClusterName = clusterName
	createdProject, err := projectsApi.CreateProject(client, &project)
	if err != nil {
		return nil, err
	}

	return createdProject, nil
}

func createProjectRoleTemplateBinding(client *rancher.Client, user *management.User, project *v3.Project, role string) (*v3.ProjectRoleTemplateBinding, error) {
	projectName := fmt.Sprintf("%s:%s", project.Namespace, project.Name)
	prtb.Name = namegen.AppendRandomString("prtb-")
	prtb.Namespace = project.Name
	prtb.ProjectName = projectName
	prtb.RoleTemplateName = role
	prtb.UserPrincipalName = user.PrincipalIDs[0]
	createdProjectRoleTemplateBinding, err := rbacApi.CreateProjectRoleTemplateBinding(client, &prtb)
	if err != nil {
		return nil, err
	}

	return createdProjectRoleTemplateBinding, nil
}

func waitForFinalizerToUpdate(client *rancher.Client, projectName string, projectNamespace string, finalizerCount int) error {
	err := kwait.Poll(timeouts.FiveHundredMillisecond, timeouts.TenSecond, func() (done bool, pollErr error) {
		project, pollErr := projectsApi.ListProjects(client, project.Namespace, metav1.ListOptions{
			FieldSelector: "metadata.name=" + project.Name,
		})
		if pollErr != nil {
			return false, pollErr
		}

		if len(project.Items[0].Finalizers) == finalizerCount {
			return true, nil
		}
		return false, nil
	})

	if err != nil {
		return err
	}

	return nil
}

func checkPodLogsForErrors(client *rancher.Client, cluster string, podName string, namespace string, errorPattern string, startTime time.Time) error {
	startTimeUTC := startTime.UTC()

	errorRegex := regexp.MustCompile(errorPattern)
	timeRegex := regexp.MustCompile(`^\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2}`)

	var errorMessage string

	kwait.Poll(timeouts.TenSecond, timeouts.TwoMinute, func() (bool, error) {
		podLogs, err := kubeconfig.GetPodLogs(client, cluster, podName, namespace, "")
		if err != nil {
			return false, err
		}

		segments := strings.Split(podLogs, "\n")
		for _, segment := range segments {
			timeMatches := timeRegex.FindStringSubmatch(segment)
			if len(timeMatches) > 0 {
				segmentTime, err := time.Parse(timeFormat, timeMatches[0])
				if err != nil {
					continue
				}

				segmentTimeUTC := segmentTime.UTC()
				if segmentTimeUTC.After(startTimeUTC) {
					if matches := errorRegex.FindStringSubmatch(segment); len(matches) > 0 {
						errorMessage = "error logs found in rancher: " + segment
						return true, nil
					}
				}
			}
		}
		return false, nil
	})

	if errorMessage != "" {
		return errors.New(errorMessage)
	}

	return nil
}

func updateProjectNamespaceFinalizer(client *rancher.Client, existingProject *v3.Project, finalizer []string) (*v3.Project, error) {
	updatedProject := existingProject.DeepCopy()
	updatedProject.ObjectMeta.Finalizers = finalizer

	updatedProject, err := projectsApi.UpdateProject(client, existingProject, updatedProject)
	if err != nil {
		return nil, err
	}

	return updatedProject, nil
}
