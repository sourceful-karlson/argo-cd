package admin

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type MockResourceGetter struct {}

func (_ MockResourceGetter) GetServerResources() []*metav1.APIResourceList {
	res := metav1.APIResource{
		Name: "services",
		Kind: "Service",
	}
	return []*metav1.APIResourceList{{APIResources: []metav1.APIResource{res}}}
}

func TestProjectAllowListGen(t *testing.T) {
	globalProj := generateProjectAllowList(MockResourceGetter{}, "testdata/test_clusterrole.yaml", "testproj")
	assert.True(t, len(globalProj.Spec.NamespaceResourceWhitelist) > 0)
}
