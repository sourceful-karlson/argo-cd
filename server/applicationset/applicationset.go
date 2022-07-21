package applicationset

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	appsetutils "github.com/argoproj/argo-cd/v2/applicationset/utils"
	"github.com/argoproj/argo-cd/v2/pkg/apiclient/applicationset"
	appv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/pkg/apis/applicationset/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/v2/pkg/client/clientset/versioned"
	applisters "github.com/argoproj/argo-cd/v2/pkg/client/listers/application/v1alpha1"
	servercache "github.com/argoproj/argo-cd/v2/server/cache"
	"github.com/argoproj/argo-cd/v2/server/rbacpolicy"
	apputil "github.com/argoproj/argo-cd/v2/util/appset"
	"github.com/argoproj/argo-cd/v2/util/db"
	"github.com/argoproj/argo-cd/v2/util/rbac"
	"github.com/argoproj/argo-cd/v2/util/settings"
	"github.com/argoproj/pkg/sync"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Server struct {
	ns           string
	db           db.ArgoDB
	enf          *rbac.Enforcer
	cache        *servercache.Cache
	appclientset appclientset.Interface
	appLister    applisters.ApplicationNamespaceLister
	projLister   applisters.AppProjectNamespaceLister
	settings     *settings.SettingsManager
	projectLock  sync.KeyLock
}

// NewServer returns a new instance of the ApplicationSet service
func NewServer(
	db db.ArgoDB,
	enf *rbac.Enforcer,
	cache *servercache.Cache,
	appclientset appclientset.Interface,
	appLister applisters.ApplicationNamespaceLister,
	projLister applisters.AppProjectNamespaceLister,
	settings *settings.SettingsManager,
	namespace string,
	projectLock sync.KeyLock,
) *Server {
	s := &Server{
		ns:           namespace,
		cache:        cache,
		db:           db,
		enf:          enf,
		appclientset: appclientset,
		appLister:    appLister,
		projLister:   projLister,
		settings:     settings,
		projectLock:  projectLock,
	}
	return s
}

// List returns list of applications
func (s *Server) List(ctx context.Context, q *applicationset.ApplicationSetQuery) (*v1alpha1.ApplicationSetList, error) {
	// labelsMap, err := labels.ConvertSelectorToLabelsMap(q.GetSelector())
	// if err != nil {
	// 	return nil, fmt.Errorf("error converting selector to labels map: %w", err)
	// }

	var appsets *v1alpha1.ApplicationSetList
	// // err = cache.ListAllByNamespace(cache.NewIndexer(), s.ns, labelsMap.AsSelector(), func(m interface{}) {
	// // 	appsets.Items = append(appsets.Items, m.(v1alpha1.ApplicationSet))
	// // })
	// if err := s.client.List(ctx, appsets, &client.ListOptions{LabelSelector: labelsMap.AsSelector()}); err != nil {
	// 	return nil, fmt.Errorf("Error fetching list of ApplicationSets")
	// }

	// if err != nil {
	// 	return nil, fmt.Errorf("error listing apps with selectors: %w", err)
	// }

	newItems := make([]v1alpha1.ApplicationSet, 0)
	for _, a := range appsets.Items {
		if s.enf.Enforce(ctx.Value("claims"), rbacpolicy.ResourceApplicationSet, rbacpolicy.ActionGet, apputil.AppSetRBACName(&a)) {
			newItems = append(newItems, a)
		}
	}
	var err error
	if q.Name != "" {
		newItems, err = filterByName(newItems, q.Name)
		if err != nil {
			return nil, fmt.Errorf("error filtering applicationsets by name: %w", err)
		}
	}

	// Sort found applicationsets by name
	sort.Slice(newItems, func(i, j int) bool {
		return newItems[i].Name < newItems[j].Name
	})

	appsetList := &v1alpha1.ApplicationSetList{
		Items: newItems,
	}

	return appsetList, nil

}

// FilterByName returns an application
func filterByName(apps []v1alpha1.ApplicationSet, name string) ([]v1alpha1.ApplicationSet, error) {
	if name == "" {
		return apps, nil
	}
	items := make([]v1alpha1.ApplicationSet, 0)
	for i := 0; i < len(apps); i++ {
		if apps[i].Name == name {
			items = append(items, apps[i])
			return items, nil
		}
	}
	return items, status.Errorf(codes.NotFound, "applicationset '%s' not found", name)
}

func (s *Server) Create(ctx context.Context, q *applicationset.ApplicationSetCreateRequest) (*v1alpha1.ApplicationSet, error) {
	if q.GetApplicationset() == nil {
		return nil, fmt.Errorf("error creating application set: applicationSet is nil in request")
	}
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceApplicationSet, rbacpolicy.ActionCreate, apputil.AppSetRBACName(q.GetApplicationset())); err != nil {
		return nil, err
	}

	appset := q.GetApplicationset()

	project, err := s.validateAppSet(ctx, appset)
	if err != nil {
		return nil, fmt.Errorf("error validating ApplicationSet: %w", err)
	}

	if err := s.checkCreatePermissions(ctx, appset, project); err != nil {
		return nil, err
	}

	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceProjects, rbacpolicy.ActionAction, project); err != nil {
		return nil, fmt.Errorf("User does not have permissions to create the applicationset %s in project %s", appset.ObjectMeta.Name, project.Name)
	}

	s.projectLock.RLock(q.GetApplicationset().Spec.Template.Spec.Project)
	defer s.projectLock.RUnlock(q.GetApplicationset().Spec.Template.Spec.Project)

	var cmd *appv1.Command
	cmd.Command = append(cmd.Command, "kubectl", "apply", "-f", q.FilePath)
	cmd.Args = append(cmd.Args, "-n", s.ns)
	createdAppSet, err := apputil.RunCommand(*cmd)
	if err != nil {
		return nil, fmt.Errorf("error creating applicationset: %w", err)
	}
	var response *v1alpha1.ApplicationSet
	err = json.Unmarshal([]byte(createdAppSet), &response)
	if err != nil {
		return nil, fmt.Errorf("error creating applicationset: %w", err)
	}

	return response, nil
}

func (s *Server) Get(ctx context.Context, q *applicationset.ApplicationSetQuery) (*v1alpha1.ApplicationSet, error) {

	return nil, nil
}

func (s *Server) Update(ctx context.Context, q *applicationset.ApplicationSetUpdateRequest) (*v1alpha1.ApplicationSet, error) {
	return nil, nil
}

func (s *Server) Delete(ctx context.Context, q *applicationset.ApplicationSetDeleteRequest) (*applicationset.ApplicationSetResponse, error) {
	return nil, nil
}

func (s *Server) validateAppSet(ctx context.Context, appset *v1alpha1.ApplicationSet) (*appv1.AppProject, error) {
	if appset == nil {
		return nil, fmt.Errorf("ApplicationSet cannot be validated for nil value")
	}

	projectName := appset.Spec.Template.Spec.Project

	if strings.HasPrefix(projectName, "{{") && strings.HasPrefix(projectName, "}}") {
		return nil, fmt.Errorf("ApplicationSet cannot have a templated value for project field")
	}

	prj, err := s.appclientset.ArgoprojV1alpha1().AppProjects(s.ns).Get(ctx, projectName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	if err := appsetutils.CheckInvalidGenerators(appset); err != nil {
		return nil, err
	}

	return prj, nil
}

func (s *Server) checkCreatePermissions(ctx context.Context, appset *v1alpha1.ApplicationSet, project *appv1.AppProject) error {

	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceProjects, rbacpolicy.ActionGet, project.Name); err != nil {
		return err
	}

	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceApplicationSet, rbacpolicy.ActionCreate, apputil.AppSetRBACName(appset)); err != nil {
		return err
	}

	return nil
}
