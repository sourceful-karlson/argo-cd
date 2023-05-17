package plugin

import (
	"context"
	"fmt"
	"net/http"

	internalhttp "github.com/argoproj/argo-cd/v2/applicationset/services/internal/http"
)

type Parameters map[string]string

// ServiceRequest is the request object sent to the plugin service.
type ServiceRequest struct {
	// ApplicationSetName is the appSetName of the ApplicationSet for which we're requesting parameters. Useful for logging in
	// the plugin service.
	ApplicationSetName string `json:"applicationSetName"`
	// Parameters is the map of parameters set in the ApplicationSet spec for this generator.
	Parameters Parameters `json:"parameters"`
}

type Service struct {
	client     *internalhttp.Client
	appSetName string
}

func NewPluginService(ctx context.Context, appSetName string, baseURL string, token string, requestTimeout int) (*Service, error) {
	var clientOptionFns []internalhttp.ClientOptionFunc

	clientOptionFns = append(clientOptionFns, internalhttp.WithToken(token))

	if requestTimeout != 0 {
		clientOptionFns = append(clientOptionFns, internalhttp.WithTimeout(requestTimeout))
	}

	client, err := internalhttp.NewClient(baseURL, clientOptionFns...)
	if err != nil {
		return nil, fmt.Errorf("error creating plugin client: %v", err)
	}

	return &Service{
		client:     client,
		appSetName: appSetName,
	}, nil
}

func (p *Service) List(ctx context.Context, parameters map[string]string) ([]map[string]interface{}, *http.Response, error) {
	req, err := p.client.NewRequest(http.MethodPost, "api/v1/getparams.execute", ServiceRequest{ApplicationSetName: p.appSetName, Parameters: parameters}, nil)

	if err != nil {
		return nil, nil, fmt.Errorf("NewRequest returned unexpected error: %v", err)
	}

	var data []map[string]interface{}

	resp, err := p.client.Do(ctx, req, &data)

	if err != nil {
		return nil, nil, fmt.Errorf("error get api '%s': %v", p.appSetName, err)
	}

	return data, resp, err
}
