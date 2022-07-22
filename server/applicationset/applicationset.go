package applicationset

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	appsetutils "github.com/argoproj/argo-cd/v2/applicationset/utils"
	"github.com/argoproj/argo-cd/v2/pkg/apiclient/applicationset"
	appv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/pkg/apis/applicationset/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/v2/pkg/client/clientset/versioned"
	applisters "github.com/argoproj/argo-cd/v2/pkg/client/listers/application/v1alpha1"
	appsetlisters "github.com/argoproj/argo-cd/v2/pkg/client/listers/applicationset/v1alpha1"
	servercache "github.com/argoproj/argo-cd/v2/server/cache"
	"github.com/argoproj/argo-cd/v2/server/rbacpolicy"
	apputil "github.com/argoproj/argo-cd/v2/util/appset"
	"github.com/argoproj/argo-cd/v2/util/argo"
	argoutil "github.com/argoproj/argo-cd/v2/util/argo"
	"github.com/argoproj/argo-cd/v2/util/db"
	"github.com/argoproj/argo-cd/v2/util/rbac"
	"github.com/argoproj/argo-cd/v2/util/session"
	"github.com/argoproj/argo-cd/v2/util/settings"
	"github.com/argoproj/pkg/sync"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	v1 "k8s.io/api/core/v1"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

type Server struct {
	ns             string
	kubeclientset  kubernetes.Interface
	db             db.ArgoDB
	enf            *rbac.Enforcer
	cache          *servercache.Cache
	appclientset   appclientset.Interface
	appLister      applisters.ApplicationNamespaceLister
	appsetInformer cache.SharedIndexInformer
	appsetLister   appsetlisters.ApplicationSetNamespaceLister
	projLister     applisters.AppProjectNamespaceLister
	auditLogger    *argo.AuditLogger
	settings       *settings.SettingsManager
	projectLock    sync.KeyLock
}

// NewServer returns a new instance of the ApplicationSet service
func NewServer(
	db db.ArgoDB,
	kubeclientset kubernetes.Interface,
	enf *rbac.Enforcer,
	cache *servercache.Cache,
	appclientset appclientset.Interface,
	appLister applisters.ApplicationNamespaceLister,
	appsetInformer cache.SharedIndexInformer,
	appsetLister appsetlisters.ApplicationSetNamespaceLister,
	projLister applisters.AppProjectNamespaceLister,
	settings *settings.SettingsManager,
	namespace string,
	projectLock sync.KeyLock,
) applicationset.ApplicationSetServiceServer {
	s := &Server{
		ns:             namespace,
		cache:          cache,
		db:             db,
		enf:            enf,
		appclientset:   appclientset,
		appLister:      appLister,
		appsetInformer: appsetInformer,
		appsetLister:   appsetLister,
		projLister:     projLister,
		settings:       settings,
		projectLock:    projectLock,
		auditLogger:    argo.NewAuditLogger(namespace, kubeclientset, "argocd-server"),
	}
	return s
}

func (s *Server) Get(ctx context.Context, q *applicationset.ApplicationSetQuery) (*v1alpha1.ApplicationSet, error) {
	return nil, nil
}

// List returns list of applications
func (s *Server) List(ctx context.Context, q *applicationset.ApplicationSetQuery) (*v1alpha1.ApplicationSetList, error) {
	labelsMap, _ := labels.ConvertSelectorToLabelsMap(q.GetSelector())
	appsets, err := s.appsetLister.List(labelsMap.AsSelector())
	if err != nil {
		return nil, fmt.Errorf("error listing apps with selectors: %w", err)
	}
	newItems := make([]v1alpha1.ApplicationSet, 0)
	for _, a := range appsets {
		if s.enf.Enforce(ctx.Value("claims"), rbacpolicy.ResourceApplicationSet, rbacpolicy.ActionGet, apputil.AppSetRBACName(a)) {
			newItems = append(newItems, *a)
		}
	}
	if q.Name != "" {
		newItems, err = argoutil.FilterAppSetsByName(newItems, q.Name)
		if err != nil {
			return nil, fmt.Errorf("error filtering applications by name: %w", err)
		}
	}

	// Filter applications by name
	newItems = argoutil.FilterAppSetsByProjects(newItems, q.Projects)

	// Sort found applications by name
	sort.Slice(newItems, func(i, j int) bool {
		return newItems[i].Name < newItems[j].Name
	})

	appsetList := v1alpha1.ApplicationSetList{
		ListMeta: metav1.ListMeta{
			ResourceVersion: s.appsetInformer.LastSyncResourceVersion(),
		},
		Items: newItems,
	}
	return &appsetList, nil

}

func (s *Server) Create(ctx context.Context, q *applicationset.ApplicationSetCreateRequest) (*v1alpha1.ApplicationSet, error) {
	if q.GetApplicationset() == nil {
		return nil, fmt.Errorf("error creating applicationset: applicationset is nil in request")
	}
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceApplicationSet, rbacpolicy.ActionCreate, apputil.AppSetRBACName(q.GetApplicationset())); err != nil {
		return nil, err
	}

	appset := q.GetApplicationset()

	project, err := s.validateAppSet(ctx, appset)
	if err != nil {
		return nil, fmt.Errorf("error validating applicationset: %w", err)
	}

	if err := s.checkCreatePermissions(ctx, appset, project); err != nil {
		return nil, err
	}

	s.projectLock.RLock(project.Name)
	defer s.projectLock.RUnlock(project.Name)

	created, err := s.appclientset.ApplicationsetV1alpha1().ApplicationSets(s.ns).Create(ctx, appset, metav1.CreateOptions{})
	if err == nil {
		s.logAppSetEvent(created, ctx, argo.EventReasonResourceCreated, "created applicationset")
		s.waitSync(created)
		return created, nil
	}

	if !apierr.IsAlreadyExists(err) {
		return nil, fmt.Errorf("error creating applicationset: %w", err)
	}
	// act idempotent if existing spec matches new spec
	existing, err := s.appsetLister.Get(appset.Name)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "unable to check existing applicationset details: %v", err)
	}

	return nil, fmt.Errorf("applicationset with name %s already exists", existing.Name)
}

func (s *Server) Update(ctx context.Context, q *applicationset.ApplicationSetUpdateRequest) (*v1alpha1.ApplicationSet, error) {

	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceApplicationSet, rbacpolicy.ActionUpdate, apputil.AppSetRBACName(q.GetApplicationset())); err != nil {
		return nil, err
	}

	validate := true
	if q.Validate {
		validate = q.Validate
	}
	return s.validateAndUpdateAppSet(ctx, q.Applicationset, false, validate)
}

func (s *Server) validateAndUpdateAppSet(ctx context.Context, newApp *v1alpha1.ApplicationSet, merge bool, validate bool) (*v1alpha1.ApplicationSet, error) {
	s.projectLock.RLock(newApp.Spec.Template.Spec.GetProject())
	defer s.projectLock.RUnlock(newApp.Spec.Template.Spec.GetProject())

	_, err := s.validateAppSet(ctx, newApp)
	if err != nil {
		return nil, fmt.Errorf("error validating appset: %w", err)
	}

	existingAppset, err := s.appclientset.ApplicationsetV1alpha1().ApplicationSets(s.ns).Get(ctx, newApp.Name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("error getting applicationset: %w", err)
	}

	if existingAppset != nil && existingAppset.Spec.Template.Spec.Project != newApp.Spec.Template.Spec.Project {
		// When changing projects, caller must have application create & update privileges in new project
		// NOTE: the update check was already verified in the caller to this function
		if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceApplicationSet, rbacpolicy.ActionCreate, apputil.AppSetRBACName(newApp)); err != nil {
			return nil, err
		}
		// They also need 'update' privileges in the old project
		if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceApplicationSet, rbacpolicy.ActionUpdate, apputil.AppSetRBACName(existingAppset)); err != nil {
			return nil, err
		}
	}

	a, err := s.updateApp(existingAppset, newApp, ctx, merge)
	if err != nil {
		return nil, fmt.Errorf("error updating applicationset: %w", err)
	}
	return a, nil
}

func mergeStringMaps(items ...map[string]string) map[string]string {
	res := make(map[string]string)
	for _, m := range items {
		if m == nil {
			continue
		}
		for k, v := range m {
			res[k] = v
		}
	}
	return res
}

func (s *Server) updateApp(appset *v1alpha1.ApplicationSet, newAppset *v1alpha1.ApplicationSet, ctx context.Context, merge bool) (*v1alpha1.ApplicationSet, error) {
	for i := 0; i < 10; i++ {
		appset.Spec = newAppset.Spec
		if merge {
			appset.Labels = mergeStringMaps(appset.Labels, newAppset.Labels)
			appset.Annotations = mergeStringMaps(appset.Annotations, newAppset.Annotations)
		} else {
			appset.Labels = newAppset.Labels
			appset.Annotations = newAppset.Annotations
		}

		res, err := s.appclientset.ApplicationsetV1alpha1().ApplicationSets(s.ns).Update(ctx, appset, metav1.UpdateOptions{})
		if err == nil {
			s.logAppSetEvent(appset, ctx, argo.EventReasonResourceUpdated, "updated applicationset spec")
			s.waitSync(res)
			return res, nil
		}
		if !apierr.IsConflict(err) {
			return nil, err
		}

		appset, err = s.appclientset.ApplicationsetV1alpha1().ApplicationSets(s.ns).Get(ctx, newAppset.Name, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("error getting application: %w", err)
		}
	}
	return nil, status.Errorf(codes.Internal, "Failed to update application. Too many conflicts")
}

func (s *Server) Delete(ctx context.Context, q *applicationset.ApplicationSetDeleteRequest) (*applicationset.ApplicationSetResponse, error) {

	appset, err := s.appclientset.ApplicationsetV1alpha1().ApplicationSets(s.ns).Get(ctx, q.Name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("error getting application: %w", err)
	}

	s.projectLock.RLock(appset.Spec.Template.Spec.Project)
	defer s.projectLock.RUnlock(appset.Spec.Template.Spec.Project)

	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceApplicationSet, rbacpolicy.ActionDelete, apputil.AppSetRBACName(appset)); err != nil {
		return nil, err
	}

	err = s.appclientset.ArgoprojV1alpha1().Applications(s.ns).Delete(ctx, q.Name, metav1.DeleteOptions{})
	if err != nil {
		return nil, fmt.Errorf("error deleting application: %w", err)
	}
	s.logAppSetEvent(appset, ctx, argo.EventReasonResourceDeleted, "deleted application")
	return &applicationset.ApplicationSetResponse{}, nil

}

func (s *Server) validateAppSet(ctx context.Context, appset *v1alpha1.ApplicationSet) (*appv1.AppProject, error) {
	if appset == nil {
		return nil, fmt.Errorf("ApplicationSet cannot be validated for nil value")
	}

	projectName := appset.Spec.Template.Spec.Project

	if strings.HasPrefix(projectName, "{{") && strings.HasSuffix(projectName, "}}") {
		return nil, fmt.Errorf("applicationset cannot have a templated value for project field")
	}

	proj, err := s.appclientset.ArgoprojV1alpha1().AppProjects(s.ns).Get(ctx, projectName, metav1.GetOptions{})
	if err != nil {
		if apierr.IsNotFound(err) {
			return nil, status.Errorf(codes.InvalidArgument, "applicationset references project %s which does not exist", projectName)
		}
		return nil, fmt.Errorf("error getting applicationset's project: %w", err)
	}

	if err := appsetutils.CheckInvalidGenerators(appset); err != nil {
		return nil, err
	}

	return proj, nil
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

var informerSyncTimeout = 2 * time.Second

// waitSync is a helper to wait until the application informer cache is synced after create/update.
// It waits until the app in the informer, has a resource version greater than the version in the
// supplied app, or after 2 seconds, whichever comes first. Returns true if synced.
// We use an informer cache for read operations (Get, List). Since the cache is only
// eventually consistent, it is possible that it doesn't reflect an application change immediately
// after a mutating API call (create/update). This function should be called after a creates &
// update to give a probable (but not guaranteed) chance of being up-to-date after the create/update.
func (s *Server) waitSync(appset *v1alpha1.ApplicationSet) {
	logCtx := log.WithField("applicationset", appset.Name)
	deadline := time.Now().Add(informerSyncTimeout)
	minVersion, err := strconv.Atoi(appset.ResourceVersion)
	if err != nil {
		logCtx.Warnf("waitSync failed: could not parse resource version %s", appset.ResourceVersion)
		time.Sleep(50 * time.Millisecond) // sleep anyway
		return
	}
	for {
		if currAppset, err := s.appsetLister.Get(appset.Name); err == nil {
			currVersion, err := strconv.Atoi(currAppset.ResourceVersion)
			if err == nil && currVersion >= minVersion {
				return
			}
		}
		if time.Now().After(deadline) {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	logCtx.Warnf("waitSync failed: timed out")
}

func (s *Server) logAppSetEvent(a *v1alpha1.ApplicationSet, ctx context.Context, reason string, action string) {
	eventInfo := argo.EventInfo{Type: v1.EventTypeNormal, Reason: reason}
	user := session.Username(ctx)
	if user == "" {
		user = "Unknown user"
	}
	message := fmt.Sprintf("%s %s", user, action)
	s.auditLogger.LogAppSetEvent(a, eventInfo, message)
}
