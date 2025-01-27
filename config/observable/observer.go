package observable

import (
	"context"
	"encoding/json"

	cfclient "github.com/cloudfoundry/go-cfclient/v3/client"
	"github.com/cloudfoundry/go-cfclient/v3/config"
	v1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	upjet "github.com/crossplane/upjet/pkg/resource"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	pathForProvider = "spec.forProvider"
	pathCredential  = "spec.credentials"
	pathEndpoint    = "spec.endpoint"
	keyExternalData = "crossplane.io/external-data"
)

// ExternalData is the interface to work with out-of-cluster data
type ExternalData interface {
	GetID() string
	Instantiate(resource.Managed, string) bool
	Read(context.Context, ConnectFn) error
}

// Observer initializes a field by observing ExternalData
type Observer struct {
	client client.Client

	// Field is the to-be-initialized tf attribute or block name to be initialized
	field string

	// dataSource is the external data
	dataSource ExternalData
}

// ObserverOption type to describe functions to manipulate Observer
type ObserverOption func(*Observer)

// NewObserver returns an Observer
func NewObserver(c client.Client, field string, ds ExternalData, opts ...ObserverOption) *Observer {
	obs := &Observer{client: c, field: field, dataSource: ds}

	for _, o := range opts {
		o(obs)
	}
	return obs
}

// Initialize implements the crossplane-runtime Initializer
func (s *Observer) Initialize(ctx context.Context, mg resource.Managed) error {
	tr, ok := mg.(upjet.Terraformed)
	if !ok {
		return errors.New("not a terraformed resource")
	}
	// get `spec.forProvider` as a map[string]any
	fp, err := tr.GetParameters()
	if err != nil {
		return errors.Wrap(err, "cannot get forProvider spec")
	}

	// Already initialized, do nothing
	if _, ok := fp[s.field]; ok {
		return nil
	}

	if !s.dataSource.Instantiate(mg, keyExternalData) {
		// return errors.New("cannot instantiate external data source")
		return nil
	}
	if err := s.dataSource.Read(ctx, NewConnectFn(s.client, mg)); err != nil {
		return err
	}

	fp[s.field] = s.dataSource.GetID()

	if err = tr.SetParameters(fp); err != nil {
		return err
	}
	return s.client.Update(ctx, tr)
}

// ConnectFn builds an external connection
type ConnectFn func(ctx context.Context) (*cfclient.Client, error)

// ConnectionDetails configures an external connection
type ConnectionDetails struct {
	Endpoint string `json:"endpoint"`
	Email    string `json:"email,omitempty"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

// NewConnectFn extract ConnectDetails from a managed resource and return a ConnectFn
func NewConnectFn(client client.Client, mg resource.Managed) ConnectFn {
	return func(ctx context.Context) (*cfclient.Client, error) {
		cfg, err := useProviderConfigRef(ctx, client, mg)
		if err != nil {
			return nil, err
		}

		cf, err := config.New(cfg.Endpoint, config.UserPassword(cfg.Email, cfg.Password), config.SkipTLSValidation())
		if err != nil {
			return nil, errors.Wrapf(err, "cannot config cloud foundry client with option: %q", cfg)
		}

		return cfclient.New(cf)
	}
}

// useProviderConfigRef borrows connection details from ProviderConfig referenced by managed resource
func useProviderConfigRef(ctx context.Context, kube client.Client, mg resource.Managed) (*ConnectionDetails, error) {
	r := mg.GetProviderConfigReference()
	if r == nil || r.Name == "" {
		return nil, errors.New("no providerconfig reference is found")
	}
	// Use unstructured as config package has no understanding of CRDs.
	// Importing any apis packages will break the upjet pipeline
	obj := &unstructured.Unstructured{}

	// GVK must match ProviderConfig of this provider
	obj.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "cloudfoundry.btp.orchestrate.cloud.sap",
		Kind:    "ProviderConfig",
		Version: "v1beta1",
	})
	if err := kube.Get(ctx, types.NamespacedName{Name: r.Name}, obj); err != nil {
		return nil, errors.Wrap(err, "cannot retrieve the referenced providerconfig object")
	}

	oc, err := getCredentials(ctx, kube, obj)
	if err != nil {
		return nil, err
	}

	url, err := getEndpoint(ctx, kube, obj)
	if err != nil {
		return nil, err
	}
	oc.Endpoint = *url

	return oc, nil
}

// getCredential extracts
func getCredentials(ctx context.Context, kube client.Client, obj *unstructured.Unstructured) (*ConnectionDetails, error) {
	v, err := fieldpath.Pave(obj.Object).GetValue(pathCredential)
	if fieldpath.IsNotFound(err) {
		return nil, errors.New("credential secret is not configured")
	}

	buf, err := extractCredentials(ctx, kube, v)
	if err != nil {
		return nil, errors.New("cannot extract credential secret")
	}
	var cfg ConnectionDetails
	err = json.Unmarshal(buf, &cfg)
	return &cfg, err
}

func getEndpoint(ctx context.Context, kube client.Client, obj *unstructured.Unstructured) (*string, error) {
	v, err := fieldpath.Pave(obj.Object).GetValue(pathEndpoint)
	if fieldpath.IsNotFound(err) {
		return nil, errors.Wrap(err, "environment secret is not configured")
	}

	buf, err := extractCredentials(ctx, kube, v)
	if err != nil {
		return nil, errors.Wrap(err, "cannot extract environment secret")
	}
	url := string(buf)
	return &url, nil
}

// CommonCredentialSpec is a helper struct for extracting credentials
type CommonCredentialSpec struct {
	Source                       v1.CredentialsSource `json:"source"`
	v1.CommonCredentialSelectors `json:",inline"`
}

func extractCredentials(ctx context.Context, kube client.Client, v any) ([]byte, error) {
	buf, err := json.Marshal(v)
	if err != nil {
		return nil, errors.Wrap(err, "cannot marshal credential config")
	}
	var cred CommonCredentialSpec
	if err := json.Unmarshal(buf, &cred); err != nil {
		return nil, errors.Wrap(err, "cannot unmarshal credential config")
	}

	return resource.CommonCredentialExtractor(ctx, cred.Source, kube, cred.CommonCredentialSelectors)
}
