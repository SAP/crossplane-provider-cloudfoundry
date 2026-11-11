package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"

	cfv3 "github.com/cloudfoundry/go-cfclient/v3/client"
	"github.com/cloudfoundry/go-cfclient/v3/config"
	"github.com/cloudfoundry/go-cfclient/v3/resource"
	"github.com/google/go-cmp/cmp"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
)

const (
	testOrgGUID     = "org-guid"
	testAppGUID     = "app-guid"
	testSpaceGUID   = "space-guid"
	testPkgGUID     = "pkg-guid"
	testBuildGUID   = "build-guid"
	testDropletGUID = "droplet-guid"
	testJobGUID     = "job-guid"
)

// fakeCF is a minimal CF API serving the requests of a docker push.
// It records the docker package requests and the final app action.
type fakeCF struct {
	mu         sync.Mutex
	packages   []resource.PackageCreate
	appState   string
	lastAction string
}

func (f *fakeCF) handler(t *testing.T, serverURL *string) http.Handler {
	mux := http.NewServeMux()
	write := func(w http.ResponseWriter, status int, body string) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}
	mux.HandleFunc("POST /oauth/token", func(w http.ResponseWriter, r *http.Request) {
		write(w, http.StatusOK, `{"token_type":"bearer","access_token":"token","refresh_token":"refresh","expires_in":3600}`)
	})
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		write(w, http.StatusOK, fmt.Sprintf(`{"links":{"login":{"href":%q},"uaa":{"href":%q}}}`, *serverURL, *serverURL))
	})
	space := fmt.Sprintf(`{"guid":%q,"name":"test-space","relationships":{"organization":{"data":{"guid":%q}}}}`, testSpaceGUID, testOrgGUID)
	org := fmt.Sprintf(`{"guid":%q,"name":"test-org"}`, testOrgGUID)
	paged := func(r string) string {
		return `{"pagination":{"total_results":1,"total_pages":1},"resources":[` + r + `]}`
	}
	mux.HandleFunc("GET /v3/spaces/"+testSpaceGUID, func(w http.ResponseWriter, r *http.Request) {
		write(w, http.StatusOK, space)
	})
	mux.HandleFunc("GET /v3/organizations/"+testOrgGUID, func(w http.ResponseWriter, r *http.Request) {
		write(w, http.StatusOK, org)
	})
	mux.HandleFunc("GET /v3/organizations", func(w http.ResponseWriter, r *http.Request) {
		write(w, http.StatusOK, paged(org))
	})
	mux.HandleFunc("GET /v3/spaces", func(w http.ResponseWriter, r *http.Request) {
		write(w, http.StatusOK, paged(space))
	})
	mux.HandleFunc("POST /v3/spaces/"+testSpaceGUID+"/actions/apply_manifest", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", *serverURL+"/v3/jobs/"+testJobGUID)
		write(w, http.StatusAccepted, "")
	})
	mux.HandleFunc("GET /v3/jobs/"+testJobGUID, func(w http.ResponseWriter, r *http.Request) {
		write(w, http.StatusOK, fmt.Sprintf(`{"guid":%q,"state":"COMPLETE"}`, testJobGUID))
	})
	mux.HandleFunc("GET /v3/apps", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		state := f.appState
		f.mu.Unlock()
		write(w, http.StatusOK, paged(fmt.Sprintf(`{"guid":%q,"name":"test-app","state":%q,"lifecycle":{"type":"docker","data":{}}}`, testAppGUID, state)))
	})
	mux.HandleFunc("POST /v3/packages", func(w http.ResponseWriter, r *http.Request) {
		var pkg resource.PackageCreate
		if err := json.NewDecoder(r.Body).Decode(&pkg); err != nil {
			t.Errorf("cannot decode package request: %v", err)
		}
		f.mu.Lock()
		f.packages = append(f.packages, pkg)
		f.mu.Unlock()
		write(w, http.StatusCreated, fmt.Sprintf(`{"guid":%q,"type":"docker","state":"READY","data":{"image":%q}}`, testPkgGUID, pkg.Data.Image))
	})
	mux.HandleFunc("POST /v3/builds", func(w http.ResponseWriter, r *http.Request) {
		write(w, http.StatusCreated, fmt.Sprintf(`{"guid":%q,"state":"STAGING"}`, testBuildGUID))
	})
	mux.HandleFunc("GET /v3/builds/"+testBuildGUID, func(w http.ResponseWriter, r *http.Request) {
		write(w, http.StatusOK, fmt.Sprintf(`{"guid":%q,"state":"STAGED"}`, testBuildGUID))
	})
	mux.HandleFunc("GET /v3/packages/"+testPkgGUID+"/droplets", func(w http.ResponseWriter, r *http.Request) {
		write(w, http.StatusOK, fmt.Sprintf(`{"pagination":{"total_results":1,"total_pages":1},"resources":[{"guid":%q,"state":"STAGED"}]}`, testDropletGUID))
	})
	mux.HandleFunc("PATCH /v3/apps/"+testAppGUID+"/relationships/current_droplet", func(w http.ResponseWriter, r *http.Request) {
		write(w, http.StatusOK, fmt.Sprintf(`{"data":{"guid":%q}}`, testDropletGUID))
	})
	mux.HandleFunc("POST /v3/apps/"+testAppGUID+"/actions/{action}", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.lastAction = r.PathValue("action")
		f.mu.Unlock()
		write(w, http.StatusOK, fmt.Sprintf(`{"guid":%q,"name":"test-app","state":"STARTED"}`, testAppGUID))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		write(w, http.StatusNotFound, "")
	})
	return mux
}

func newFakeCFPushClient(t *testing.T, f *fakeCF) *pushClient {
	t.Helper()
	var serverURL string
	srv := httptest.NewServer(f.handler(t, &serverURL))
	t.Cleanup(srv.Close)
	serverURL = srv.URL

	cfg, err := config.New(srv.URL, config.Token("", "refresh"))
	if err != nil {
		t.Fatalf("config.New() error = %v", err)
	}
	cf, err := cfv3.New(cfg)
	if err != nil {
		t.Fatalf("client.New() error = %v", err)
	}
	return NewPushClient(cf)
}

func testApp() *resource.App {
	return &resource.App{
		Resource: resource.Resource{GUID: testAppGUID},
		Relationships: resource.AppRelationships{
			Space: resource.ToOneRelationship{Data: &resource.Relationship{GUID: testSpaceGUID}},
		},
	}
}

func testDockerSpec() v1alpha1.AppParameters {
	return v1alpha1.AppParameters{
		Name:   "test-app",
		Docker: &v1alpha1.DockerConfiguration{Image: "registry.example.com/team/app:1.0"},
	}
}

func TestPushDockerCredentials(t *testing.T) {
	tests := []struct {
		name        string
		credentials *DockerCredentials
		envPassword string
		want        *resource.DockerCredentials
	}{
		{
			name:        "credentials are sent with the docker package",
			credentials: &DockerCredentials{Username: "user", Password: "secret"},
			want:        &resource.DockerCredentials{Username: "user", Password: "secret"},
		},
		{
			name:        "CF_DOCKER_PASSWORD from another reconcile is ignored",
			credentials: &DockerCredentials{Username: "user", Password: "secret"},
			envPassword: "other-tenant-secret",
			want:        &resource.DockerCredentials{Username: "user", Password: "secret"},
		},
		{
			name:        "no credentials are sent for a public image",
			envPassword: "other-tenant-secret",
			want:        nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("CF_DOCKER_PASSWORD", tt.envPassword)
			f := &fakeCF{appState: "STOPPED"}
			p := newFakeCFPushClient(t, f)

			spec := testDockerSpec()
			manifest, err := newManifestFromSpec(spec, tt.credentials)
			if err != nil {
				t.Fatalf("newManifestFromSpec() error = %v", err)
			}
			if _, err := p.Push(context.Background(), testApp(), manifest, tt.credentials); err != nil {
				t.Fatalf("Push() error = %v", err)
			}

			if len(f.packages) != 1 {
				t.Fatalf("expected 1 package request, got %d", len(f.packages))
			}
			pkg := f.packages[0]
			if pkg.Type != "docker" {
				t.Errorf("package type = %q, want docker", pkg.Type)
			}
			if pkg.Data == nil {
				t.Fatal("package data is missing")
			}
			if pkg.Data.Image != spec.Docker.Image {
				t.Errorf("package image = %q, want %q", pkg.Data.Image, spec.Docker.Image)
			}
			if diff := cmp.Diff(tt.want, pkg.Data.DockerCredentials); diff != "" {
				t.Errorf("package credentials mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestPushStartsOrRestartsApp(t *testing.T) {
	tests := map[string]string{
		"STOPPED": "start",
		"STARTED": "restart",
	}
	for state, wantAction := range tests {
		t.Run(state, func(t *testing.T) {
			f := &fakeCF{appState: state}
			p := newFakeCFPushClient(t, f)
			manifest, err := newManifestFromSpec(testDockerSpec(), nil)
			if err != nil {
				t.Fatalf("newManifestFromSpec() error = %v", err)
			}
			if _, err := p.Push(context.Background(), testApp(), manifest, nil); err != nil {
				t.Fatalf("Push() error = %v", err)
			}
			if f.lastAction != wantAction {
				t.Errorf("app action = %q, want %q", f.lastAction, wantAction)
			}
		})
	}
}

func TestConfigDockerDoesNotSetEnv(t *testing.T) {
	t.Setenv("CF_DOCKER_PASSWORD", "")
	creds := &DockerCredentials{Username: "user", Password: "secret"}
	docker, err := configDocker(testDockerSpec(), creds)
	if err != nil {
		t.Fatalf("configDocker() error = %v", err)
	}
	if docker.Username != "user" {
		t.Errorf("docker username = %q, want user", docker.Username)
	}
	if v := os.Getenv("CF_DOCKER_PASSWORD"); v != "" {
		t.Errorf("CF_DOCKER_PASSWORD = %q, want it unset", v)
	}
}
