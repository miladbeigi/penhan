package backends

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// newFakeKV2Server serves KV v2 data reads and writes from memory. Paths in
// softDeleted answer like a deleted version: 200 with null data.
func newFakeKV2Server(t *testing.T, softDeleted ...string) (srv *httptest.Server, store map[string]map[string]interface{}) {
	t.Helper()
	var mu sync.Mutex
	store = map[string]map[string]interface{}{}
	deleted := map[string]bool{}
	for _, p := range softDeleted {
		deleted[p] = true
	}

	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		path := strings.TrimPrefix(r.URL.Path, "/v1/")

		switch r.Method {
		case http.MethodPut, http.MethodPost:
			var body struct {
				Data map[string]interface{} `json:"data"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			store[path] = body.Data
			_, _ = w.Write([]byte(`{"data":{"version":1}}`))
		case http.MethodGet:
			if deleted[path] {
				_, _ = w.Write([]byte(`{"data":{"data":null,"metadata":{"deletion_time":"2026-01-01T00:00:00Z"}}}`))
				return
			}
			data, ok := store[path]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`{"errors":[]}`))
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"data": map[string]interface{}{"data": data}})
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, store
}

func newTestVault(t *testing.T, addr, basePath string) *VaultProvider {
	t.Helper()
	p, err := NewVaultProvider(VaultOptions{Addr: addr, Token: "test-token", MountPath: "secret", BasePath: basePath})
	if err != nil {
		t.Fatalf("NewVaultProvider() error = %v", err)
	}
	return p
}

func TestVaultPushPull(t *testing.T) {
	srv, store := newFakeKV2Server(t)
	p := newTestVault(t, srv.URL, "myapp")

	if err := p.Push([]byte(`{"password":"hunter2"}`), "db/main"); err != nil {
		t.Fatalf("Push() error = %v", err)
	}
	if store["secret/data/myapp/db/main"]["password"] != "hunter2" {
		t.Fatalf("secret not written under {mount}/data/{base}/{path}: %v", store)
	}

	got, err := p.Pull("db/main")
	if err != nil {
		t.Fatalf("Pull() error = %v", err)
	}
	if string(got) != `{"password":"hunter2"}` {
		t.Errorf("Pull() = %s", got)
	}
}

func TestVaultPullMissing(t *testing.T) {
	srv, _ := newFakeKV2Server(t, "secret/data/gone")
	p := newTestVault(t, srv.URL, "")

	for _, path := range []string{"never-written", "gone"} {
		if _, err := p.Pull(path); !errors.Is(err, ErrNotFound) {
			t.Errorf("Pull(%q) error = %v, want ErrNotFound", path, err)
		}
	}
}

func TestVaultPushRejectsNonJSON(t *testing.T) {
	srv, _ := newFakeKV2Server(t)
	if err := newTestVault(t, srv.URL, "").Push([]byte("not json"), "x"); err == nil {
		t.Fatal("Push() must reject content that is not a JSON object")
	}
}

func TestVaultBuildPath(t *testing.T) {
	cases := []struct{ base, path, want string }{
		{"", "db", "secret/data/db"},
		{"myapp", "db", "secret/data/myapp/db"},
		{"myapp", "apps/api", "secret/data/myapp/apps/api"},
	}
	for _, tc := range cases {
		p := &VaultProvider{mountPath: "secret", basePath: tc.base}
		if got := p.buildPath(tc.path); got != tc.want {
			t.Errorf("buildPath(%q) with base %q = %q, want %q", tc.path, tc.base, got, tc.want)
		}
	}
}
