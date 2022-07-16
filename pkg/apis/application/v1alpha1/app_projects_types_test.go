package v1alpha1

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func Test_checkRestrictedBy(t *testing.T) {
	t.Run("no parent, nothing to do", func(t *testing.T) {
		isPermitted, err := checkRestrictedBy(
			&AppProject{},
			func(name string) (*AppProject, error) {
				t.Fatal("checkRestrictedBy tried to get a project, which it shouldn't do, because the root project has no parents")
				return nil, nil
			},
			func(project *AppProject) (bool, error) {
				t.Fatal("checkRestrictedBy tried to check a project, which it shouldn't do, because the root project has no parents, and the root project is not checked by checkRestrictedBy")
				return false, nil
			},
		)
		assert.NoError(t, err)
		assert.True(t, isPermitted)
	})
	t.Run("one parent, no errors", func(t *testing.T) {
		testCases := []bool{false, true}

		for _, testCase := range testCases {
			testCase := testCase

			t.Run(fmt.Sprintf("permitted %v", testCase), func(t *testing.T) {
				isPermitted, err := checkRestrictedBy(
					&AppProject{Spec: AppProjectSpec{RestrictedBy: []string{"parent"}}},
					func(name string) (*AppProject, error) {
						return &AppProject{}, nil
					},
					func(project *AppProject) (bool, error) {
						return testCase, nil
					},
				)
				assert.NoError(t, err)
				assert.Equal(t, testCase, isPermitted)
			})
		}
	})
	t.Run("one parent, error getting parent", func(t *testing.T) {
		expectedError := errors.New("failed to get parent project")
		isPermitted, err := checkRestrictedBy(
			&AppProject{Spec: AppProjectSpec{RestrictedBy: []string{"parent"}}},
			func(name string) (*AppProject, error) {
				return nil, expectedError
			},
			func(project *AppProject) (bool, error) {
				return true, nil
			},
		)
		assert.ErrorIs(t, err, expectedError)
		assert.False(t, isPermitted)
	})
	t.Run("one parent, error checking", func(t *testing.T) {
		expectedError := errors.New("failed to check")
		isPermitted, err := checkRestrictedBy(
			&AppProject{Spec: AppProjectSpec{RestrictedBy: []string{"parent"}}},
			func(name string) (*AppProject, error) {
				return &AppProject{}, nil
			},
			func(project *AppProject) (bool, error) {
				return false, expectedError
			},
		)
		assert.ErrorIs(t, err, expectedError)
		assert.False(t, isPermitted)
	})
	t.Run("loop", func(t *testing.T) {
		isPermitted, err := checkRestrictedBy(
			&AppProject{ObjectMeta: v1.ObjectMeta{Name: "a"}, Spec: AppProjectSpec{RestrictedBy: []string{"b"}}},
			func(name string) (*AppProject, error) {
				if name == "a" {
					t.Fatal("checkProject looped back and checked the initial project - it should not do that")
				} else if name == "b" {
					return &AppProject{ObjectMeta: v1.ObjectMeta{Name: "b"}, Spec: AppProjectSpec{RestrictedBy: []string{"a"}}}, nil
				} else {
					t.Fatalf("checkProject tried to get project %q that wasn't referenced", name)
				}
				return nil, nil  // this shouldn't happen
			},
			func(project *AppProject) (bool, error) {
				return true, nil
			},
		)
		assert.NoError(t, err)
		assert.True(t, isPermitted)
	})
	t.Run("self-referential", func(t *testing.T) {
		isPermitted, err := checkRestrictedBy(
			&AppProject{ObjectMeta: v1.ObjectMeta{Name: "root"}, Spec: AppProjectSpec{RestrictedBy: []string{"root"}}},
			func(name string) (*AppProject, error) {
				t.Fatal("checkRestrictedBy tried to get a project, which it shouldn't do, because the root project has no parents")
				return nil, nil
			},
			func(project *AppProject) (bool, error) {
				t.Fatal("checkRestrictedBy tried to check a project, which it shouldn't do, because the root project has no parents, and the root project is not checked by checkRestrictedBy")
				return false, nil
			},
		)
		assert.NoError(t, err)
		assert.True(t, isPermitted)
	})
	t.Run("all Projects must be visited with no double-visits, tree with no loops", func(t *testing.T) {
		projects := map[string]*AppProject{
			"a": {
				ObjectMeta: v1.ObjectMeta{Name: "a"},
				Spec: AppProjectSpec{RestrictedBy: []string{"b", "e"}},
			},
			"b": {
				ObjectMeta: v1.ObjectMeta{Name: "b"},
				Spec: AppProjectSpec{RestrictedBy: []string{"c", "d"}},
			},
			"c": {
				ObjectMeta: v1.ObjectMeta{Name: "c"},
			},
			"d": {
				ObjectMeta: v1.ObjectMeta{Name: "d"},
			},
			"e": {
				ObjectMeta: v1.ObjectMeta{Name: "e"},
				Spec: AppProjectSpec{RestrictedBy: []string{"f", "g"}},
			},
			"f": {
				ObjectMeta: v1.ObjectMeta{Name: "f"},
			},
			"g": {
				ObjectMeta: v1.ObjectMeta{Name: "g"},
			},
		}
		checkedProjects := map[string]bool{}
		isPermitted, err := checkRestrictedBy(
			projects["a"],
			func(name string) (*AppProject, error) {
				if checkedProjects[name] {
					t.Fatalf("checkRestrictedBy tried to get project %q even though it's already been checked", name)
				}
				return projects[name], nil
			},
			func(project *AppProject) (bool, error) {
				checkedProjects[project.Name] = true
				return true, nil
			},
		)
		assert.NoError(t, err)
		assert.True(t, isPermitted)
		assert.Len(t, checkedProjects, len(projects) - 1, "should have checked all but the root project")
	})
	t.Run("all Projects must be visited with no double-visits, tree with a loop", func(t *testing.T) {
		projects := map[string]*AppProject{
			"a": {
				ObjectMeta: v1.ObjectMeta{Name: "a"},
				Spec: AppProjectSpec{RestrictedBy: []string{"b", "e"}},
			},
			"b": {
				ObjectMeta: v1.ObjectMeta{Name: "b"},
				Spec: AppProjectSpec{RestrictedBy: []string{"c", "d"}},
			},
			"c": {
				ObjectMeta: v1.ObjectMeta{Name: "c"},
			},
			"d": {
				ObjectMeta: v1.ObjectMeta{Name: "d"},
			},
			"e": {
				ObjectMeta: v1.ObjectMeta{Name: "e"},
				Spec: AppProjectSpec{RestrictedBy: []string{"f", "g"}},
			},
			"f": {
				ObjectMeta: v1.ObjectMeta{Name: "f"},
			},
			"g": {
				ObjectMeta: v1.ObjectMeta{Name: "g"},
				Spec: AppProjectSpec{RestrictedBy: []string{"c"}},
			},
		}
		checkedProjects := map[string]bool{}
		isPermitted, err := checkRestrictedBy(
			projects["a"],
			func(name string) (*AppProject, error) {
				if checkedProjects[name] {
					t.Fatalf("checkRestrictedBy tried to get project %q even though it's already been checked", name)
				}
				return projects[name], nil
			},
			func(project *AppProject) (bool, error) {
				checkedProjects[project.Name] = true
				return true, nil
			},
		)
		assert.NoError(t, err)
		assert.True(t, isPermitted)
		assert.Len(t, checkedProjects, len(projects) - 1, "should have checked all but the root project")
	})
}

func Test_IsGroupKindPermitted(t *testing.T) {
	t.Run("parent projects block resources when child does not", func(t *testing.T) {
		projects := map[string]*AppProject{
			"root": {
				ObjectMeta: v1.ObjectMeta{
					Name: "root",
				},
				Spec: AppProjectSpec{
					RestrictedBy: []string{"b"},
				},
			},
			"b": {
				ObjectMeta: v1.ObjectMeta{
					Name: "b",
				},
				Spec: AppProjectSpec{
					RestrictedBy: []string{"c"},
					NamespaceResourceBlacklist: []v1.GroupKind{
						{Group: "v1", Kind: "ConfigMap"},
					},
				},
			},
			"c": {
				ObjectMeta: v1.ObjectMeta{
					Name: "c",
				},
				Spec: AppProjectSpec{
					NamespaceResourceBlacklist: []v1.GroupKind{
						{Group: "v1", Kind: "ServiceAccount"},
					},
				},
			},
		}

		getProject := func(name string) (*AppProject, error) {
			// No restrictions in the child app.
			project, ok := projects[name]
			if !ok {
				return nil, fmt.Errorf("failed to get project %q", name)
			}
			return project, nil
		}

		t.Run("ConfigMap should be blocked by project 'b'", func(t *testing.T) {
			isPermitted, err := projects["root"].IsGroupKindPermitted(schema.GroupKind{Group: "v1", Kind: "ConfigMap"}, true, getProject)
			assert.NoError(t, err)
			assert.False(t, isPermitted)
		})

		t.Run("ServiceAccount should be blocked by project 'c'", func(t *testing.T) {
			isPermitted, err := projects["root"].IsGroupKindPermitted(schema.GroupKind{Group: "v1", Kind: "ServiceAccount"}, true, getProject)
			assert.NoError(t, err)
			assert.False(t, isPermitted)
		})

		t.Run("Pod is not blocked", func(t *testing.T) {
			isPermitted, err := projects["root"].IsGroupKindPermitted(schema.GroupKind{Group: "v1", Kind: "Pod"}, true, getProject)
			assert.NoError(t, err)
			assert.True(t, isPermitted)
		})

		projects["root"].Spec.RestrictedBy = []string{"does-not-exist"}

		t.Run("error caused by non-existent project in chain", func(t *testing.T) {
			isPermitted, err := projects["root"].IsGroupKindPermitted(schema.GroupKind{Group: "v1", Kind: "Pod"}, true, getProject)
			assert.Error(t, err)
			assert.False(t, isPermitted)
		})
	})
}

func Test_IsDestinationPermitted(t *testing.T) {
	t.Run("parent projects block destination when child does not", func(t *testing.T) {
		projects := map[string]*AppProject{
			"root": {
				ObjectMeta: v1.ObjectMeta{
					Name: "root",
				},
				Spec: AppProjectSpec{
					RestrictedBy: []string{"b"},
					Destinations: []ApplicationDestination{
						{Server: "*", Name: "*", Namespace: "*"},
					},
				},
			},
			"b": {
				ObjectMeta: v1.ObjectMeta{
					Name: "b",
				},
				Spec: AppProjectSpec{
					RestrictedBy: []string{"c"},
					Destinations: []ApplicationDestination{
						{Name: "dev-usw2-*", Namespace: "dev-usw2-*"},
					},
				},
			},
			"c": {
				ObjectMeta: v1.ObjectMeta{
					Name: "c",
				},
				Spec: AppProjectSpec{
					Destinations: []ApplicationDestination{
						{Name: "dev-*", Namespace: "dev-*"},
					},
				},
			},
		}

		getProject := func(name string) (*AppProject, error) {
			// No restrictions in the child app.
			project, ok := projects[name]
			if !ok {
				return nil, fmt.Errorf("failed to get project %q", name)
			}
			return project, nil
		}

		t.Run("use2 cluster/ns is blocked by project 'b'", func(t *testing.T) {
			isPermitted, err := projects["root"].IsDestinationPermitted(ApplicationDestination{Name: "dev-use2-cluster", Namespace: "dev-use2-namespace"}, getProject)
			assert.NoError(t, err)
			assert.False(t, isPermitted)
		})

		t.Run("usw2 is allowed by the whole chain", func(t *testing.T) {
			isPermitted, err := projects["root"].IsDestinationPermitted(ApplicationDestination{Name: "dev-usw2-cluster", Namespace: "dev-usw2-namespace"}, getProject)
			assert.NoError(t, err)
			assert.True(t, isPermitted)
		})

		projects["root"].Spec.RestrictedBy = []string{"does-not-exist"}

		t.Run("error because parent project does not exist", func(t *testing.T) {
			isPermitted, err := projects["root"].IsDestinationPermitted(ApplicationDestination{Name: "dev-usw2-cluster", Namespace: "dev-usw2-namespace"}, getProject)
			assert.Error(t, err)
			assert.False(t, isPermitted)
		})
	})
}

func Test_IsSourcePermitted(t *testing.T) {
	t.Run("parent projects block source when child does not", func(t *testing.T) {
		projects := map[string]*AppProject{
			"root": {
				ObjectMeta: v1.ObjectMeta{
					Name: "root",
				},
				Spec: AppProjectSpec{
					RestrictedBy: []string{"b"},
					SourceRepos: []string{
						"https://github.company.com/dev-org/*",
					},
				},
			},
			"b": {
				ObjectMeta: v1.ObjectMeta{
					Name: "b",
				},
				Spec: AppProjectSpec{
					RestrictedBy: []string{"c"},
					SourceRepos: []string{
						"https://github.company.com/dev-org/*-deployment.git",
					},
				},
			},
			"c": {
				ObjectMeta: v1.ObjectMeta{
					Name: "c",
				},
				Spec: AppProjectSpec{
					SourceRepos: []string{
						"https://github.company.com/*/*",
					},
				},
			},
		}

		getProject := func(name string) (*AppProject, error) {
			// No restrictions in the child app.
			project, ok := projects[name]
			if !ok {
				return nil, fmt.Errorf("failed to get project %q", name)
			}
			return project, nil
		}

		t.Run("repo URL allowed by full chain", func(t *testing.T) {
			isPermitted, err := projects["root"].IsSourcePermitted(ApplicationSource{RepoURL: "https://github.company.com/dev-org/app1-deployment.git"}, getProject)
			assert.NoError(t, err)
			assert.True(t, isPermitted)
		})

		t.Run("other-org blocked by chain", func(t *testing.T) {
			isPermitted, err := projects["root"].IsSourcePermitted(ApplicationSource{RepoURL: "https://github.company.com/other-org/app1-deployment.git"}, getProject)
			assert.NoError(t, err)
			assert.False(t, isPermitted)
		})

		projects["root"].Spec.RestrictedBy = []string{"does-not-exist"}

		t.Run("error because the parent project does not exist", func(t *testing.T) {
			isPermitted, err := projects["root"].IsSourcePermitted(ApplicationSource{RepoURL: "https://github.company.com/other-org/app1-deployment.git"}, getProject)
			assert.Error(t, err)
			assert.False(t, isPermitted)
		})
	})
}

func Test_IsResourcePermitted(t *testing.T) {
	t.Run("parent projects block source when child does not", func(t *testing.T) {
		projects := map[string]*AppProject{
			"root": {
				ObjectMeta: v1.ObjectMeta{
					Name: "root",
				},
				Spec: AppProjectSpec{
					RestrictedBy: []string{"b"},
					Destinations: []ApplicationDestination{
						{Name: "*", Namespace: "*"},
					},
					ClusterResourceWhitelist: []v1.GroupKind{
						{Group: "*", Kind: "*"},
					},
				},
			},
			"b": {
				ObjectMeta: v1.ObjectMeta{
					Name: "b",
				},
				Spec: AppProjectSpec{
					RestrictedBy: []string{"c"},
					Destinations: []ApplicationDestination{
						{Name: "*", Namespace: "*"},
					},
					NamespaceResourceBlacklist: []v1.GroupKind{
						{Group: "v1", Kind: "ConfigMap"},
					},
					ClusterResourceWhitelist: []v1.GroupKind{
						{Group: "v1", Kind: "Pod"}, // not actually cluster-scoped, but it works for the test
					},
				},
			},
			"c": {
				ObjectMeta: v1.ObjectMeta{
					Name: "c",
				},
				Spec: AppProjectSpec{
					Destinations: []ApplicationDestination{
						{Name: "dev-usw2-*", Namespace: "dev-usw2-*"},
					},
					ClusterResourceWhitelist: []v1.GroupKind{
						{Group: "*", Kind: "*"},
					},
				},
			},
		}

		getProject := func(name string) (*AppProject, error) {
			// No restrictions in the child app.
			project, ok := projects[name]
			if !ok {
				return nil, fmt.Errorf("failed to get project %q", name)
			}
			return project, nil
		}

		allowedGroupKind := schema.GroupKind{Group: "v1", Kind: "Pod"}
		disallowedGroupKind := schema.GroupKind{Group: "v1", Kind: "ConfigMap"}
		allowedNamespace := "dev-usw2-namespace"
		disallowedNamespace := "dev-use2-namespace"
		allowedDestination := ApplicationDestination{Name: "dev-usw2-cluster"}
		disallowedDestination := ApplicationDestination{Name: "dev-use2-cluster"}

		for _, groupKind := range []schema.GroupKind{allowedGroupKind, disallowedGroupKind} {
			for _, namespace := range []string{allowedNamespace, disallowedNamespace} {
				for _, destination := range []ApplicationDestination{allowedDestination, disallowedDestination} {
					groupKindAllowed := groupKind == allowedGroupKind
					namespaceAllowed := namespace == allowedNamespace
					destinationAllowed := destination == allowedDestination


					destWithNamespace := ApplicationDestination{Name: destination.Name, Namespace: namespace}

					t.Run(fmt.Sprintf("GroupKind %v allowed: %v, namespace %s allowed: %v, destination %v allowed: %v", groupKind, groupKindAllowed, namespace, namespaceAllowed, destination, destinationAllowed), func(t *testing.T) {
						isPermitted, err := projects["root"].IsResourcePermitted(groupKind, namespace, destWithNamespace, getProject)
						resourceAllowed := groupKindAllowed && namespaceAllowed && destinationAllowed
						assert.NoError(t, err)
						assert.Equal(t, resourceAllowed, isPermitted)
					})

					t.Run(fmt.Sprintf("GroupKind %v allowed: %v, destination %v allowed: %v", groupKind, groupKindAllowed, destination, destinationAllowed), func(t *testing.T) {
						// test for cluster-scoped resource
						isPermitted, err := projects["root"].IsResourcePermitted(groupKind, "", destination, getProject)
						// Namespace isn't checked for cluster resources, and destination server is checked at the Application level instead of the resource level.
						clusterResourceAllowed := groupKindAllowed
						assert.NoError(t, err)
						assert.Equal(t, clusterResourceAllowed, isPermitted)
					})
				}
			}
		}

		projects["root"].Spec.RestrictedBy = []string{"does-not-exist"}

		t.Run("error getting parent project", func(t *testing.T) {
			isPermitted, err := projects["root"].IsResourcePermitted(allowedGroupKind, allowedNamespace, allowedDestination, getProject)
			assert.Error(t, err)
			assert.False(t, isPermitted)
		})
	})

	t.Run("compose a few restrictions", func(t *testing.T) {
		projects := map[string]*AppProject{
			"no-configmaps": {
				ObjectMeta: v1.ObjectMeta{
					Name: "no-configmaps",
				},
				Spec: AppProjectSpec{
					Destinations: []ApplicationDestination{
						{Name: "*", Namespace: "*"},
					},
					NamespaceResourceBlacklist: []v1.GroupKind{
						{"v1", "ConfigMap"},
					},
					ClusterResourceWhitelist: []v1.GroupKind{
						{"v1", "Pod"},
					},
				},
			},
			"dev-clusters-only": {
				ObjectMeta: v1.ObjectMeta{
					Name: "dev-clusters-only",
				},
				Spec: AppProjectSpec{
					Destinations: []ApplicationDestination{
						{Name: "dev-*", Namespace: "*"},
					},
				},
			},
			"dev-namespaces-only": {
				ObjectMeta: v1.ObjectMeta{
					Name: "dev-namespaces-only",
				},
				Spec: AppProjectSpec{
					Destinations: []ApplicationDestination{
						{Name: "*", Namespace: "dev-*"},
					},
				},
			},
			"no-configmaps-dev-only": {
				ObjectMeta: v1.ObjectMeta{
					Name: "no-configmaps-dev-only",
				},
				Spec: AppProjectSpec{
					Destinations: []ApplicationDestination{
						{Name: "*", Namespace: "*"},
					},
					RestrictedBy: []string{
						"no-configmaps",
						"dev-clusters-only",
						"dev-namespaces-only",
					},
				},
			},
		}

		getProject := func(name string) (*AppProject, error) {
			// No restrictions in the child app.
			project, ok := projects[name]
			if !ok {
				return nil, fmt.Errorf("failed to get project %q", name)
			}
			return project, nil
		}

		allowedGroupKind := schema.GroupKind{Group: "v1", Kind: "Pod"}
		disallowedGroupKind := schema.GroupKind{Group: "v1", Kind: "ConfigMap"}
		allowedDestination := ApplicationDestination{Name: "dev-usw2-cluster", Namespace: "dev-usw2-namespace"}
		disallowedDestination := ApplicationDestination{Name: "prod-usw2-cluster", Namespace: "prod-usw2-namespace"}

		for _, groupKind := range []schema.GroupKind{allowedGroupKind, disallowedGroupKind} {
			for _, destination := range []ApplicationDestination{allowedDestination, disallowedDestination} {
				groupKindAllowed := groupKind == allowedGroupKind
				destinationAllowed := destination == allowedDestination

				t.Run(fmt.Sprintf("GroupKind %v allowed: %v, destination %v allowed: %v", groupKind, groupKindAllowed, destination, destinationAllowed), func(t *testing.T) {
					isPermitted, err := projects["no-configmaps-dev-only"].IsResourcePermitted(groupKind, destination.Namespace, destination, getProject)
					resourceAllowed := groupKindAllowed && destinationAllowed
					assert.NoError(t, err)
					assert.Equal(t, resourceAllowed, isPermitted)
				})
			}
		}
	})
}
