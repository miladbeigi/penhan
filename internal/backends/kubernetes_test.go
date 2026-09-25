package backends

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
)

// newTestKubernetes returns a provider for safe "myapp" in namespace "apps".
func newTestKubernetes(t *testing.T, objects ...runtime.Object) (*KubernetesProvider, *fake.Clientset) {
	t.Helper()
	client := fake.NewClientset(objects...)
	secrets := fakeSecrets{client.CoreV1().Secrets("apps")}
	return &KubernetesProvider{secrets: secrets, namespace: "apps", safe: "myapp"}, client
}

// fakeSecrets adapts client-go's fake clientset to secretClient. The fake
// only lives in tests, so its full API scheme never reaches the binary.
type fakeSecrets struct {
	api corev1client.SecretInterface
}

func (f fakeSecrets) List(ctx context.Context) ([]corev1.Secret, error) {
	list, err := f.api.List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

func (f fakeSecrets) Get(ctx context.Context, name string) (*corev1.Secret, error) {
	return f.api.Get(ctx, name, metav1.GetOptions{})
}

func (f fakeSecrets) Create(ctx context.Context, s *corev1.Secret) error {
	_, err := f.api.Create(ctx, s, metav1.CreateOptions{})
	return err
}

func (f fakeSecrets) Update(ctx context.Context, s *corev1.Secret) error {
	_, err := f.api.Update(ctx, s, metav1.UpdateOptions{})
	return err
}

func getSecret(t *testing.T, client *fake.Clientset, name string) *corev1.Secret {
	t.Helper()
	s, err := client.CoreV1().Secrets("apps").Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get secret %s: %v", name, err)
	}
	return s
}

func TestSecretName(t *testing.T) {
	valid := map[string]string{
		"db":             "db",
		"db/password":    "db-password",
		"apps/api/token": "apps-api-token",
		"tls.prod":       "tls.prod",
	}
	for path, want := range valid {
		got, err := SecretName(path)
		if err != nil || got != want {
			t.Errorf("SecretName(%q) = %q, %v; want %q", path, got, err, want)
		}
	}
	for _, path := range []string{"DB", "db_password", "-db", "db-", strings.Repeat("a", 254), "with space"} {
		if _, err := SecretName(path); err == nil {
			t.Errorf("SecretName(%q) should be rejected", path)
		}
	}
}

func TestKubernetesPushCreatesManagedSecret(t *testing.T) {
	p, client := newTestKubernetes(t)

	if err := p.Push([]byte(`{"password":"hunter2","user":"app"}`), "db/main"); err != nil {
		t.Fatalf("Push() error = %v", err)
	}

	s := getSecret(t, client, "db-main")
	if s.Type != corev1.SecretTypeOpaque {
		t.Errorf("type = %q, want Opaque", s.Type)
	}
	if string(s.Data["password"]) != "hunter2" || string(s.Data["user"]) != "app" {
		t.Errorf("data = %v", s.Data)
	}
	if s.Labels[managedByLabel] != managedByValue {
		t.Errorf("missing managed-by label: %v", s.Labels)
	}
	if s.Annotations[safeAnnotation] != "myapp" || s.Annotations[pathAnnotation] != "db/main" {
		t.Errorf("annotations = %v", s.Annotations)
	}
}

func TestKubernetesPushPullRoundtrip(t *testing.T) {
	p, _ := newTestKubernetes(t)
	if err := p.Push([]byte(`{"k":"v"}`), "db"); err != nil {
		t.Fatal(err)
	}
	got, err := p.Pull("db")
	if err != nil {
		t.Fatalf("Pull() error = %v", err)
	}
	if string(got) != `{"k":"v"}` {
		t.Errorf("Pull() = %s", got)
	}
}

func TestKubernetesPushReplacesData(t *testing.T) {
	p, client := newTestKubernetes(t)
	if err := p.Push([]byte(`{"a":"1","b":"2"}`), "db"); err != nil {
		t.Fatal(err)
	}
	if err := p.Push([]byte(`{"a":"changed"}`), "db"); err != nil {
		t.Fatalf("second Push() error = %v", err)
	}

	s := getSecret(t, client, "db")
	if string(s.Data["a"]) != "changed" {
		t.Errorf("a = %q, want changed", s.Data["a"])
	}
	if _, ok := s.Data["b"]; ok {
		t.Error("a key removed locally must be removed from the Secret")
	}
}

// Labels and annotations added by others (e.g. a GitOps tool) survive a push.
func TestKubernetesPushKeepsForeignMetadata(t *testing.T) {
	p, client := newTestKubernetes(t)
	if err := p.Push([]byte(`{"a":"1"}`), "db"); err != nil {
		t.Fatal(err)
	}
	s := getSecret(t, client, "db")
	s.Labels["team"] = "payments"
	if _, err := client.CoreV1().Secrets("apps").Update(context.Background(), s, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}

	if err := p.Push([]byte(`{"a":"2"}`), "db"); err != nil {
		t.Fatal(err)
	}
	if got := getSecret(t, client, "db").Labels["team"]; got != "payments" {
		t.Errorf("foreign label lost, got %q", got)
	}
}

func TestKubernetesPullMissing(t *testing.T) {
	p, _ := newTestKubernetes(t)
	if _, err := p.Pull("nope"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Pull() error = %v, want ErrNotFound", err)
	}
}

func TestKubernetesRefusesSecretsItDoesNotOwn(t *testing.T) {
	secret := func(name string, labels, annotations map[string]string) *corev1.Secret {
		return &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "apps", Labels: labels, Annotations: annotations},
			Data:       map[string][]byte{"k": []byte("original")},
		}
	}
	managed := map[string]string{managedByLabel: managedByValue}

	cases := []struct {
		name   string
		object *corev1.Secret
		path   string
		want   string
	}{
		{"unmanaged", secret("db", nil, nil), "db", "not managed by penhan"},
		{"other safe", secret("db", managed, map[string]string{safeAnnotation: "other", pathAnnotation: "db"}), "db", `belongs to safe "other"`},
		{"path collision", secret("db-main", managed, map[string]string{safeAnnotation: "myapp", pathAnnotation: "db-main"}), "db/main", "maps to the same name"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p, client := newTestKubernetes(t, tc.object)

			if _, err := p.Pull(tc.path); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("Pull() error = %v, want %q", err, tc.want)
			}
			if err := p.Push([]byte(`{"k":"new"}`), tc.path); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("Push() error = %v, want %q", err, tc.want)
			}
			if got := getSecret(t, client, tc.object.Name).Data["k"]; string(got) != "original" {
				t.Errorf("secret was modified: %q", got)
			}
		})
	}
}

func TestKubernetesPushRejectsInvalidInput(t *testing.T) {
	p, _ := newTestKubernetes(t)
	if err := p.Push([]byte(`{"k":"v"}`), "Uppercase"); err == nil {
		t.Error("Push() must reject a path that is not a valid Secret name")
	}
	if err := p.Push([]byte(`{"bad key":"v"}`), "db"); err == nil || !strings.Contains(err.Error(), "bad key") {
		t.Errorf("Push() must reject an invalid data key, got %v", err)
	}
	if err := p.Push([]byte(`not json`), "db"); err == nil {
		t.Error("Push() must reject non-JSON content")
	}
}

func TestNewKubernetesProvider(t *testing.T) {
	if _, err := NewKubernetesProvider(KubernetesOptions{Safe: "myapp"}); err == nil {
		t.Error("expected error for missing namespace")
	}
	if _, err := NewKubernetesProvider(KubernetesOptions{Namespace: "apps"}); err == nil {
		t.Error("expected error for missing safe")
	}

	kubeconfig := filepath.Join(t.TempDir(), "config")
	if err := os.WriteFile(kubeconfig, []byte(testKubeconfig), 0o600); err != nil {
		t.Fatal(err)
	}
	opts := KubernetesOptions{Kubeconfig: kubeconfig, Context: "dev", Namespace: "apps", Safe: "myapp"}
	if _, err := NewKubernetesProvider(opts); err != nil {
		t.Fatalf("NewKubernetesProvider() error = %v", err)
	}

	// A pinned context missing from the kubeconfig must fail, never fall
	// back to whatever the current context is.
	opts.Context = "prod"
	if _, err := NewKubernetesProvider(opts); err == nil {
		t.Error("expected error for a context missing from the kubeconfig")
	}
}

const testKubeconfig = `apiVersion: v1
kind: Config
current-context: dev
clusters:
- name: dev
  cluster:
    server: https://127.0.0.1:6443
contexts:
- name: dev
  context:
    cluster: dev
    user: dev
users:
- name: dev
  user:
    token: test
`

func importFixture(name string, mutate func(*corev1.Secret)) *corev1.Secret {
	s := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "apps"},
		Type:       corev1.SecretTypeOpaque,
		Data:       map[string][]byte{"k": []byte("v")},
	}
	if mutate != nil {
		mutate(s)
	}
	return s
}

func TestKubernetesImportCandidates(t *testing.T) {
	isController := true
	p, _ := newTestKubernetes(t,
		importFixture("plain", nil),
		importFixture("pull", func(s *corev1.Secret) { s.Type = corev1.SecretTypeDockerConfigJson }),
		importFixture("helm", func(s *corev1.Secret) {
			s.Annotations = map[string]string{"meta.helm.sh/release-name": "karakeep"}
		}),
		importFixture("argo", func(s *corev1.Secret) {
			s.Labels = map[string]string{"argocd.argoproj.io/secret-type": "repository"}
		}),
		importFixture("owned", func(s *corev1.Secret) {
			s.OwnerReferences = []metav1.OwnerReference{{Kind: "Node", Name: "n1", Controller: &isController}}
		}),
		importFixture("other-tool", func(s *corev1.Secret) {
			s.Labels = map[string]string{managedByLabel: "sealed-secrets"}
		}),
		importFixture("other-safe", func(s *corev1.Secret) {
			s.Labels = map[string]string{managedByLabel: managedByValue}
			s.Annotations = map[string]string{safeAnnotation: "beta", pathAnnotation: "other-safe"}
		}),
		importFixture("mine", func(s *corev1.Secret) {
			s.Labels = map[string]string{managedByLabel: managedByValue}
			s.Annotations = map[string]string{safeAnnotation: "myapp", pathAnnotation: "mine"}
		}),
		importFixture("binary", func(s *corev1.Secret) { s.Data["bin"] = []byte{0xff, 0xfe} }),
	)

	got, err := p.ImportCandidates()
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"argo":       "Argo CD",
		"binary":     "binary data",
		"helm":       "Helm release karakeep",
		"mine":       "",
		"other-safe": `safe "beta"`,
		"other-tool": "sealed-secrets",
		"owned":      "owned by Node n1",
		"plain":      "",
		"pull":       "not supported",
	}
	if len(got) != len(want) {
		t.Fatalf("got %d candidates, want %d: %+v", len(got), len(want), got)
	}
	for i, c := range got {
		if i > 0 && got[i-1].Name > c.Name {
			t.Errorf("candidates not sorted: %s before %s", got[i-1].Name, c.Name)
		}
		w := want[c.Name]
		if (w == "") != (c.Reason == "") || !strings.Contains(c.Reason, w) {
			t.Errorf("%s: reason = %q, want %q", c.Name, c.Reason, w)
		}
	}
}

func TestKubernetesImportAdoptsWithoutChangingData(t *testing.T) {
	fixture := importFixture("tunnel", func(s *corev1.Secret) {
		s.Labels = map[string]string{"app": "cloudflared"}
		s.Data = map[string][]byte{"credentials.json": []byte(`{"a":"b"}`), "token": []byte("0012")}
	})
	p, client := newTestKubernetes(t, fixture)

	path, content, err := p.ReadForImport("tunnel")
	if err != nil {
		t.Fatal(err)
	}
	if path != "tunnel" || string(content) != `{"credentials.json":"{\"a\":\"b\"}","token":"0012"}` {
		t.Fatalf("ReadForImport() = %q, %s", path, content)
	}
	if err := p.Adopt(path, content); err != nil {
		t.Fatalf("Adopt() error = %v", err)
	}

	s := getSecret(t, client, "tunnel")
	if string(s.Data["token"]) != "0012" || len(s.Data) != 2 {
		t.Errorf("data changed: %v", s.Data)
	}
	if s.Labels["app"] != "cloudflared" || s.Labels[managedByLabel] != managedByValue {
		t.Errorf("labels = %v", s.Labels)
	}
	if s.Annotations[safeAnnotation] != "myapp" || s.Annotations[pathAnnotation] != "tunnel" {
		t.Errorf("annotations = %v", s.Annotations)
	}

	// Once adopted, it is an ordinary managed secret.
	if got, err := p.Pull("tunnel"); err != nil || string(got) != string(content) {
		t.Errorf("Pull() after adopt = %s, %v", got, err)
	}
}

func TestKubernetesAdoptRefusesChangedData(t *testing.T) {
	p, client := newTestKubernetes(t, importFixture("db", nil))
	path, content, err := p.ReadForImport("db")
	if err != nil {
		t.Fatal(err)
	}

	s := getSecret(t, client, "db")
	s.Data["k"] = []byte("changed-in-cluster")
	if _, err := client.CoreV1().Secrets("apps").Update(context.Background(), s, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}

	if err := p.Adopt(path, content); err == nil || !strings.Contains(err.Error(), "changed during import") {
		t.Fatalf("Adopt() error = %v, want changed during import", err)
	}
	if getSecret(t, client, "db").Labels[managedByLabel] == managedByValue {
		t.Error("a Secret that changed during import must not be adopted")
	}
}

func TestKubernetesReadForImportRefusesBlocked(t *testing.T) {
	p, _ := newTestKubernetes(t, importFixture("pull", func(s *corev1.Secret) { s.Type = corev1.SecretTypeDockerConfigJson }))
	if _, _, err := p.ReadForImport("pull"); err == nil || !strings.Contains(err.Error(), "not supported") {
		t.Errorf("ReadForImport() error = %v", err)
	}
	if _, _, err := p.ReadForImport("missing"); err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("ReadForImport(missing) error = %v", err)
	}
}
