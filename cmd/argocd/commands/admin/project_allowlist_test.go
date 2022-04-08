package admin

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetServerResources() func() ([]*metav1.APIResourceList, error) {
	return func() ([]*metav1.APIResourceList, error) {
		res := metav1.APIResource{
			Name: "services",
			Kind: "Service",
		}
		return []*metav1.APIResourceList{{APIResources: []metav1.APIResource{res}}}, nil
	}
}

func TestProjectAllowListGen(t *testing.T) {
	globalProj := generateProjectAllowList(GetServerResources(), "testdata/test_clusterrole.yaml", "testproj")
	assert.True(t, len(globalProj.Spec.NamespaceResourceWhitelist) > 0)
}
