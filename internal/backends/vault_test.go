package backends

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestVaultProviderSetup(t *testing.T) {
	provider := NewVaultProvider()

	err := provider.Setup(SetupOptions{Addr: "https://vault.example.com", Token: "test-token", MountPath: "secret"})
	if err != nil {
		t.Fatalf("Setup() error = %v", err)
	}

	if !provider.IsInitialized() {
		t.Error("IsInitialized() = false, want true")
	}
}

// newFakeKV2Server serves the KV v2 metadata LIST and data GET endpoints for a
// fixed tree: two top-level secrets and one nested under a folder.
func newFakeKV2Server(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	listResponses := map[string][]string{
		"/v1/secret/metadata":            {"apps/", "db"},
		"/v1/secret/metadata/apps":       {"api-token", "inner/"},
		"/v1/secret/metadata/apps/inner": {"deep"},
	}
	lookup := func(p string) ([]string, bool) {
		keys, ok := listResponses[strings.TrimSuffix(p, "/")]
		return keys, ok
	}
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		isList := r.Method == "LIST" || (r.Method == "GET" && r.URL.Query().Get("list") == "true")
		if isList {
			keys, ok := lookup(r.URL.Path)
			if !ok {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`{"errors":[]}`))
				return
			}
			w.Header().Set("Content-Type", "application/json")
			resp := map[string]interface{}{"data": map[string]interface{}{"keys": keys}}
			if err := json.NewEncoder(w).Encode(resp); err != nil {
				t.Errorf("encode list response: %v", err)
			}
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"errors":[]}`))
	})
	return httptest.NewServer(mux)
}

// KV v2 folders must be traversed: List must return every secret in the tree,
// never bare folder names.
func TestListRecursesIntoFolders(t *testing.T) {
	srv := newFakeKV2Server(t)
	defer srv.Close()

	provider := NewVaultProvider()
	if err := provider.Setup(SetupOptions{Addr: srv.URL, Token: "test-token", MountPath: "secret"}); err != nil {
		t.Fatalf("Setup() error = %v", err)
	}

	paths, err := provider.List("")
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	sort.Strings(paths)
	want := []string{"apps/api-token", "apps/inner/deep", "db"}
	if !reflect.DeepEqual(paths, want) {
		t.Errorf("List() = %v, want %v", paths, want)
	}
}
