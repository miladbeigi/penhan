//go:build e2e

package e2e

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/testcontainers/testcontainers-go/modules/k3s"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

const k3sImage = "rancher/k3s:v1.34.1-k3s1"

// k8sEnv is one throwaway k3s cluster. Booting k3s takes tens of seconds, so
// scenarios share a cluster and isolate themselves by namespace instead.
type k8sEnv struct {
	kubeconfig string // path to a kubeconfig whose current context is "default"
	client     kubernetes.Interface
}

func startK3s(t *testing.T) k8sEnv {
	t.Helper()
	ctx := context.Background()
	c, err := k3s.Run(ctx, k3sImage)
	if err != nil {
		t.Fatalf("start k3s container: %v", err)
	}
	t.Cleanup(func() { _ = c.Terminate(ctx) })

	raw, err := c.GetKubeConfig(ctx)
	if err != nil {
		t.Fatalf("k3s kubeconfig: %v", err)
	}
	path := filepath.Join(t.TempDir(), "kubeconfig")
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	restConfig, err := clientcmd.RESTConfigFromKubeConfig(raw)
	if err != nil {
		t.Fatalf("parse k3s kubeconfig: %v", err)
	}
	client, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		t.Fatalf("kubernetes client: %v", err)
	}
	return k8sEnv{kubeconfig: path, client: client}
}

func (k k8sEnv) createNamespace(t *testing.T, name string) {
	t.Helper()
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name}}
	if _, err := k.client.CoreV1().Namespaces().Create(context.Background(), ns, metav1.CreateOptions{}); err != nil {
		t.Fatalf("create namespace %s: %v", name, err)
	}
}

// secret returns the Secret, or nil when it does not exist.
func (k k8sEnv) secret(t *testing.T, namespace, name string) *corev1.Secret {
	t.Helper()
	s, err := k.client.CoreV1().Secrets(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		t.Fatalf("get secret %s/%s: %v", namespace, name, err)
	}
	return s
}

func (k k8sEnv) updateSecret(t *testing.T, s *corev1.Secret) {
	t.Helper()
	if _, err := k.client.CoreV1().Secrets(s.Namespace).Update(context.Background(), s, metav1.UpdateOptions{}); err != nil {
		t.Fatalf("update secret %s/%s: %v", s.Namespace, s.Name, err)
	}
}

// addK8sSafe runs `penhan add <name>` non-interactively against namespace.
func addK8sSafe(t *testing.T, dir, name, namespace string, k k8sEnv) string {
	t.Helper()
	stdout, stderr, code := run(t, dir, "add", name,
		"--encryption=aes",
		"--backend=kubernetes",
		"--kubeconfig="+k.kubeconfig,
		"--kube-namespace="+namespace,
	)
	requireSuccess(t, "add "+name, stdout, stderr, code)
	return filepath.Join(dir, name)
}

func TestKubernetesBackend(t *testing.T) {
	k := startK3s(t)

	// The full single-user flow: encrypted files in git, pushed as native
	// Secrets that workloads can mount.
	t.Run("journey", func(t *testing.T) {
		k.createNamespace(t, "journey")
		safe := addK8sSafe(t, newProject(t), "myapp", "journey", k)

		cfg := readFile(t, safe, "penhan.yaml")
		for _, want := range []string{"type: kubernetes", "context: default", "namespace: journey", "safe: myapp"} {
			requireContains(t, "penhan.yaml", cfg, want)
		}
		requireNoFile(t, safe, ".penhan/vault-token")

		writeFile(t, safe, "secrets/db.yaml", "password: hunter2\nuser: app\n")
		writeFile(t, safe, "secrets/apps/api.yaml", "token: abc123\n")
		stdout, stderr, code := run(t, safe, "encrypt")
		requireSuccess(t, "encrypt", stdout, stderr, code)
		requireNoFile(t, safe, "secrets/db.yaml")

		stdout, stderr, code = run(t, safe, "check")
		requireSuccess(t, "check", stdout, stderr, code)
		requireContains(t, "check", stdout, "new        apps/api")
		requireContains(t, "check", stdout, "new        db")
		if k.secret(t, "journey", "db") != nil {
			t.Fatal("check must not write to the cluster")
		}

		stdout, stderr, code = run(t, safe, "push")
		requireSuccess(t, "push", stdout, stderr, code)
		requireContains(t, "push", stdout, "Push complete: 2 pushed, 0 unchanged")

		db := k.secret(t, "journey", "db")
		if db == nil || string(db.Data["password"]) != "hunter2" || string(db.Data["user"]) != "app" {
			t.Fatalf("db Secret has wrong data: %v", db)
		}
		if db.Type != corev1.SecretTypeOpaque || db.Labels["app.kubernetes.io/managed-by"] != "penhan" {
			t.Errorf("db Secret type/labels = %s %v", db.Type, db.Labels)
		}
		if db.Annotations["penhan/safe"] != "myapp" || db.Annotations["penhan/path"] != "db" {
			t.Errorf("db Secret annotations = %v", db.Annotations)
		}
		if api := k.secret(t, "journey", "apps-api"); api == nil || string(api.Data["token"]) != "abc123" {
			t.Fatalf("nested path should become Secret apps-api, got %v", api)
		}

		stdout, stderr, code = run(t, safe, "push")
		requireSuccess(t, "second push", stdout, stderr, code)
		requireContains(t, "second push", stdout, "Push complete: 0 pushed, 2 unchanged")

		// A local edit that drops a key: push replaces the Secret's data.
		stdout, stderr, code = run(t, safe, "decrypt", "secrets/db.yaml.enc")
		requireSuccess(t, "decrypt", stdout, stderr, code)
		writeFile(t, safe, "secrets/db.yaml", "password: rotated\n")
		stdout, stderr, code = run(t, safe, "push")
		requireSuccess(t, "push after edit", stdout, stderr, code)
		requireContains(t, "push after edit", stdout, "Pushed (changed): db")
		db = k.secret(t, "journey", "db")
		if string(db.Data["password"]) != "rotated" {
			t.Errorf("password = %q, want rotated", db.Data["password"])
		}
		if _, ok := db.Data["user"]; ok {
			t.Error("a key removed locally must be removed from the Secret")
		}

		// An edit made directly in the cluster is detected and overwritten.
		db.Data["password"] = []byte("edited-in-cluster")
		k.updateSecret(t, db)
		stdout, stderr, code = run(t, safe, "check")
		requireSuccess(t, "check after drift", stdout, stderr, code)
		requireContains(t, "check after drift", stdout, "changed    db")
		stdout, stderr, code = run(t, safe, "push")
		requireSuccess(t, "push after drift", stdout, stderr, code)
		if got := k.secret(t, "journey", "db").Data["password"]; string(got) != "rotated" {
			t.Errorf("push should restore the local value, got %q", got)
		}
	})

	// penhan must never overwrite a Secret it did not create, including one
	// created by another safe in the same namespace.
	t.Run("ownership", func(t *testing.T) {
		k.createNamespace(t, "shared")
		foreign := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "tls", Namespace: "shared"},
			Data:       map[string][]byte{"cert": []byte("do-not-touch")},
		}
		if _, err := k.client.CoreV1().Secrets("shared").Create(context.Background(), foreign, metav1.CreateOptions{}); err != nil {
			t.Fatal(err)
		}

		dir := newProject(t)
		alpha := addK8sSafe(t, dir, "alpha", "shared", k)
		beta := addK8sSafe(t, dir, "beta", "shared", k)

		writeFile(t, alpha, "secrets/tls.yaml", "cert: mine\n")
		stdout, stderr, code := run(t, alpha, "push")
		if code == 0 {
			t.Fatalf("push over an unmanaged Secret must fail:\n%s", stdout)
		}
		requireContains(t, "push", stderr, "not managed by penhan")
		if got := k.secret(t, "shared", "tls").Data["cert"]; string(got) != "do-not-touch" {
			t.Errorf("unmanaged Secret was modified: %q", got)
		}

		if err := os.Remove(filepath.Join(alpha, "secrets", "tls.yaml")); err != nil {
			t.Fatal(err)
		}
		writeFile(t, alpha, "secrets/db.yaml", "owner: alpha\n")
		stdout, stderr, code = run(t, alpha, "push")
		requireSuccess(t, "alpha push", stdout, stderr, code)

		writeFile(t, beta, "secrets/db.yaml", "owner: beta\n")
		_, stderr, code = run(t, beta, "check")
		if code == 0 {
			t.Fatal("check must report a Secret owned by another safe")
		}
		requireContains(t, "beta check", stderr, `belongs to safe "alpha"`)
		_, _, code = run(t, beta, "push")
		if code == 0 {
			t.Fatal("push must not take over another safe's Secret")
		}
		if got := k.secret(t, "shared", "db").Data["owner"]; string(got) != "alpha" {
			t.Errorf("alpha's Secret was overwritten: %q", got)
		}
	})

	// Secret paths that are not valid Kubernetes names fail before any write.
	t.Run("invalid_name", func(t *testing.T) {
		k.createNamespace(t, "names")
		safe := addK8sSafe(t, newProject(t), "names", "names", k)
		writeFile(t, safe, "secrets/DB_Password.yaml", "k: v\n")

		_, stderr, code := run(t, safe, "push")
		if code == 0 {
			t.Fatal("push must reject a path that is not a valid Secret name")
		}
		requireContains(t, "push", stderr, "DB_Password")
		requireContains(t, "push", stderr, "lowercase")
	})

	// The context recorded by add wins over whatever kubectl is switched to
	// later, so a push can never land in another cluster.
	t.Run("pinned_context", func(t *testing.T) {
		k.createNamespace(t, "pinned")
		dir := newProject(t)
		kubeconfig := filepath.Join(dir, "kubeconfig")
		raw, err := os.ReadFile(k.kubeconfig)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(kubeconfig, raw, 0o600); err != nil {
			t.Fatal(err)
		}

		stdout, stderr, code := run(t, dir, "add", "pin",
			"--encryption=aes", "--backend=kubernetes",
			"--kubeconfig="+kubeconfig, "--kube-namespace=pinned",
		)
		requireSuccess(t, "add", stdout, stderr, code)
		safe := filepath.Join(dir, "pin")

		switchCurrentContext(t, kubeconfig, "elsewhere", "https://127.0.0.1:1")

		writeFile(t, safe, "secrets/db.yaml", "k: v\n")
		stdout, stderr, code = run(t, safe, "push")
		requireSuccess(t, "push", stdout, stderr, code)
		if k.secret(t, "pinned", "db") == nil {
			t.Fatal("push should target the pinned context's cluster")
		}
	})

	t.Run("add_requires_namespace", func(t *testing.T) {
		stdout, stderr, code := run(t, newProject(t), "add", "nons",
			"--encryption=aes", "--backend=kubernetes", "--kubeconfig="+k.kubeconfig)
		if code == 0 {
			t.Fatalf("add without a namespace must fail:\n%s", stdout)
		}
		requireContains(t, "add", stderr, "--kube-namespace")
	})

	t.Run("add_rejects_unknown_context", func(t *testing.T) {
		dir := newProject(t)
		stdout, stderr, code := run(t, dir, "add", "noctx",
			"--encryption=aes", "--backend=kubernetes", "--kubeconfig="+k.kubeconfig,
			"--kube-namespace=default", "--kube-context=nope")
		if code == 0 {
			t.Fatalf("add with an unknown context must fail:\n%s", stdout)
		}
		requireContains(t, "add", stderr, `no context "nope"`)
		requireNoFile(t, dir, "noctx")
	})
}

// switchCurrentContext adds a context pointing at server and makes it current,
// as `kubectl config use-context` would.
func switchCurrentContext(t *testing.T, kubeconfig, name, server string) {
	t.Helper()
	cfg, err := clientcmd.LoadFromFile(kubeconfig)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Clusters[name] = &clientcmdapi.Cluster{Server: server}
	cfg.AuthInfos[name] = &clientcmdapi.AuthInfo{Token: "x"}
	cfg.Contexts[name] = &clientcmdapi.Context{Cluster: name, AuthInfo: name}
	cfg.CurrentContext = name
	if err := clientcmd.WriteToFile(*cfg, kubeconfig); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(readFile(t, filepath.Dir(kubeconfig), filepath.Base(kubeconfig)), "current-context: "+name) {
		t.Fatal("failed to switch the current context")
	}
}
