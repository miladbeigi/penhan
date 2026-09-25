package backends

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Metadata penhan sets on every Secret it writes. Push and Pull only touch
// Secrets carrying all three with matching values, so penhan never
// overwrites a Secret it does not own.
const (
	managedByLabel = "app.kubernetes.io/managed-by"
	managedByValue = "penhan"
	safeAnnotation = "penhan/safe"
	pathAnnotation = "penhan/path"
)

// kubernetesTimeout bounds each API request, so an unreachable cluster fails
// instead of hanging.
const kubernetesTimeout = 30 * time.Second

// KubernetesOptions configures a KubernetesProvider.
type KubernetesOptions struct {
	Kubeconfig string // kubeconfig path; empty uses $KUBECONFIG or ~/.kube/config
	Context    string // kubeconfig context; empty uses the current context
	Namespace  string
	Safe       string // owning safe, recorded on each Secret
}

// KubernetesProvider stores each secret as an Opaque Secret in one namespace,
// with one Secret data key per secret key, so workloads can consume them
// directly. The Secret name is the secret path with "/" replaced by "-".
type KubernetesProvider struct {
	secrets   secretClient
	namespace string
	safe      string
}

// secretClient is the slice of the Secrets API penhan uses, scoped to one
// namespace.
type secretClient interface {
	List(ctx context.Context) ([]corev1.Secret, error)
	Get(ctx context.Context, name string) (*corev1.Secret, error)
	Create(ctx context.Context, secret *corev1.Secret) error
	Update(ctx context.Context, secret *corev1.Secret) error
}

func NewKubernetesProvider(opts KubernetesOptions) (*KubernetesProvider, error) {
	if opts.Namespace == "" {
		return nil, fmt.Errorf("kubernetes backend: namespace is required")
	}
	if opts.Safe == "" {
		return nil, fmt.Errorf("kubernetes backend: safe name is required")
	}

	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	rules.ExplicitPath = opts.Kubeconfig
	overrides := &clientcmd.ConfigOverrides{CurrentContext: opts.Context}
	restConfig, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, overrides).ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("load kubeconfig: %w", err)
	}
	restConfig.Timeout = kubernetesTimeout

	client, err := newRESTSecrets(restConfig, opts.Namespace)
	if err != nil {
		return nil, fmt.Errorf("create kubernetes client: %w", err)
	}
	return &KubernetesProvider{secrets: client, namespace: opts.Namespace, safe: opts.Safe}, nil
}

// SecretName maps a secret path to its Kubernetes Secret name. Paths that do
// not produce a valid name are rejected rather than rewritten, since
// rewriting (e.g. lowercasing) could map two files to the same Secret.
func SecretName(path string) (string, error) {
	name := strings.ReplaceAll(path, "/", "-")
	if errs := validation.IsDNS1123Subdomain(name); len(errs) > 0 {
		return "", fmt.Errorf("secret path %q maps to Kubernetes Secret name %q, which is invalid (%s); rename the file using lowercase letters, digits, '-' and '.'",
			path, name, strings.Join(errs, "; "))
	}
	return name, nil
}

func (p *KubernetesProvider) Push(content []byte, path string) error {
	name, err := SecretName(path)
	if err != nil {
		return err
	}

	var kv map[string]string
	if err := json.Unmarshal(content, &kv); err != nil {
		return fmt.Errorf("unmarshal secret data: %w", err)
	}
	data := make(map[string][]byte, len(kv))
	for k, v := range kv {
		if errs := validation.IsConfigMapKey(k); len(errs) > 0 {
			return fmt.Errorf("secret %q: key %q is not a valid Kubernetes Secret key (%s)", path, k, strings.Join(errs, "; "))
		}
		data[k] = []byte(v)
	}

	ctx, cancel := context.WithTimeout(context.Background(), kubernetesTimeout)
	defer cancel()

	existing, err := p.secrets.Get(ctx, name)
	if apierrors.IsNotFound(err) {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: p.namespace},
			Type:       corev1.SecretTypeOpaque,
			Data:       data,
		}
		p.setOwnership(secret, path)
		if err := p.secrets.Create(ctx, secret); err != nil {
			return fmt.Errorf("create secret %s: %w", p.ref(name), err)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("get secret %s: %w", p.ref(name), err)
	}

	if err := p.checkOwnership(existing, path); err != nil {
		return err
	}
	// Replace the data wholesale so keys removed locally disappear too.
	existing.Data = data
	existing.StringData = nil
	p.setOwnership(existing, path)
	if err := p.secrets.Update(ctx, existing); err != nil {
		return fmt.Errorf("update secret %s: %w", p.ref(name), err)
	}
	return nil
}

func (p *KubernetesProvider) Pull(path string) ([]byte, error) {
	name, err := SecretName(path)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), kubernetesTimeout)
	defer cancel()

	secret, err := p.secrets.Get(ctx, name)
	if apierrors.IsNotFound(err) {
		return nil, fmt.Errorf("%w: %s", ErrNotFound, path)
	}
	if err != nil {
		return nil, fmt.Errorf("get secret %s: %w", p.ref(name), err)
	}
	if err := p.checkOwnership(secret, path); err != nil {
		return nil, err
	}

	return secretContent(secret)
}

// ImportCandidate is one Secret in the namespace and whether it can be imported.
type ImportCandidate struct {
	Name   string
	Keys   int
	Reason string // why it cannot be imported; empty when it can
}

// ImportCandidates lists every Secret in the namespace, sorted by name.
func (p *KubernetesProvider) ImportCandidates() ([]ImportCandidate, error) {
	ctx, cancel := context.WithTimeout(context.Background(), kubernetesTimeout)
	defer cancel()

	items, err := p.secrets.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list secrets in %s: %w", p.namespace, err)
	}
	out := make([]ImportCandidate, 0, len(items))
	for i := range items {
		s := &items[i]
		out = append(out, ImportCandidate{Name: s.Name, Keys: len(s.Data), Reason: p.importBlocker(s)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// ReadForImport returns the content of Secret name, and the secret path it
// maps to, if penhan may take it over.
func (p *KubernetesProvider) ReadForImport(name string) (path string, content []byte, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), kubernetesTimeout)
	defer cancel()

	secret, err := p.secrets.Get(ctx, name)
	if apierrors.IsNotFound(err) {
		return "", nil, fmt.Errorf("secret %s does not exist", p.ref(name))
	}
	if err != nil {
		return "", nil, fmt.Errorf("get secret %s: %w", p.ref(name), err)
	}
	if reason := p.importBlocker(secret); reason != "" {
		return "", nil, fmt.Errorf("cannot import %s: %s", p.ref(name), reason)
	}
	content, err = secretContent(secret)
	return name, content, err
}

// Adopt marks Secret path as managed by this safe, changing only its labels
// and annotations. It fails if the data no longer equals content, so a
// change made in the cluster during import is never silently reverted by a
// later push.
func (p *KubernetesProvider) Adopt(path string, content []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), kubernetesTimeout)
	defer cancel()

	secret, err := p.secrets.Get(ctx, path)
	if err != nil {
		return fmt.Errorf("get secret %s: %w", p.ref(path), err)
	}
	if reason := p.importBlocker(secret); reason != "" {
		return fmt.Errorf("cannot import %s: %s", p.ref(path), reason)
	}
	current, err := secretContent(secret)
	if err != nil {
		return err
	}
	if !bytes.Equal(current, content) {
		return fmt.Errorf("secret %s changed during import; run import again", p.ref(path))
	}

	p.setOwnership(secret, path)
	if err := p.secrets.Update(ctx, secret); err != nil {
		return fmt.Errorf("mark secret %s as managed by penhan: %w", p.ref(path), err)
	}
	return nil
}

// importBlocker explains why penhan must not take over secret, or returns ""
// when it may: an Opaque Secret with text values that no other tool, and no
// other safe, manages.
func (p *KubernetesProvider) importBlocker(secret *corev1.Secret) string {
	if secret.Type != "" && secret.Type != corev1.SecretTypeOpaque {
		return fmt.Sprintf("type %s is not supported (penhan manages Opaque Secrets only)", secret.Type)
	}
	if refs := secret.OwnerReferences; len(refs) > 0 {
		return fmt.Sprintf("owned by %s %s", refs[0].Kind, refs[0].Name)
	}
	if release := secret.Annotations["meta.helm.sh/release-name"]; release != "" {
		return fmt.Sprintf("managed by Helm release %s", release)
	}
	for k := range secret.Labels {
		if strings.HasPrefix(k, "argocd.argoproj.io/") {
			return "managed by Argo CD"
		}
	}
	for k := range secret.Annotations {
		if strings.HasPrefix(k, "argocd.argoproj.io/") {
			return "managed by Argo CD"
		}
	}
	if m := secret.Labels[managedByLabel]; m != "" && m != managedByValue {
		return "managed by " + m
	}
	if m := secret.Labels[managedByLabel]; m == managedByValue {
		if owner := secret.Annotations[safeAnnotation]; owner != p.safe {
			return fmt.Sprintf("belongs to safe %q", owner)
		}
		if path := secret.Annotations[pathAnnotation]; path != secret.Name {
			return fmt.Sprintf("already managed as %q", path)
		}
	}
	for k, v := range secret.Data {
		if !utf8.Valid(v) {
			return fmt.Sprintf("key %q holds binary data, which secret files cannot represent", k)
		}
	}
	return ""
}

// secretContent encodes a Secret's data as penhan's canonical JSON content.
func secretContent(secret *corev1.Secret) ([]byte, error) {
	kv := make(map[string]string, len(secret.Data))
	for k, v := range secret.Data {
		kv[k] = string(v)
	}
	return json.Marshal(kv)
}

func (p *KubernetesProvider) setOwnership(secret *corev1.Secret, path string) {
	if secret.Labels == nil {
		secret.Labels = map[string]string{}
	}
	if secret.Annotations == nil {
		secret.Annotations = map[string]string{}
	}
	secret.Labels[managedByLabel] = managedByValue
	secret.Annotations[safeAnnotation] = p.safe
	secret.Annotations[pathAnnotation] = path
}

// checkOwnership reports why penhan must not touch secret on behalf of path,
// or nil when the Secret is this safe's copy of path.
func (p *KubernetesProvider) checkOwnership(secret *corev1.Secret, path string) error {
	ref := p.ref(secret.Name)
	if secret.Labels[managedByLabel] != managedByValue {
		return fmt.Errorf("secret %s exists but is not managed by penhan; refusing to overwrite it (delete it or rename %q)", ref, path)
	}
	if owner := secret.Annotations[safeAnnotation]; owner != p.safe {
		return fmt.Errorf("secret %s belongs to safe %q, not %q; use a different namespace or rename %q", ref, owner, p.safe, path)
	}
	if other := secret.Annotations[pathAnnotation]; other != path {
		return fmt.Errorf("secret %s already holds %q, which maps to the same name as %q; rename one of them", ref, other, path)
	}
	return nil
}

func (p *KubernetesProvider) ref(name string) string {
	return p.namespace + "/" + name
}

// restSecrets talks to the core/v1 Secrets endpoint through a REST client
// that knows only core/v1 types. The generated typed clients register every
// Kubernetes API group, which roughly triples the size of the penhan binary.
type restSecrets struct {
	client    rest.Interface
	namespace string
}

func newRESTSecrets(restConfig *rest.Config, namespace string) (*restSecrets, error) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		return nil, err
	}
	cfg := rest.CopyConfig(restConfig)
	cfg.APIPath = "/api"
	cfg.GroupVersion = &corev1.SchemeGroupVersion
	cfg.NegotiatedSerializer = serializer.NewCodecFactory(scheme).WithoutConversion()
	if cfg.UserAgent == "" {
		cfg.UserAgent = "penhan"
	}

	client, err := rest.RESTClientFor(cfg)
	if err != nil {
		return nil, err
	}
	return &restSecrets{client: client, namespace: namespace}, nil
}

func (r *restSecrets) List(ctx context.Context) ([]corev1.Secret, error) {
	list := &corev1.SecretList{}
	if err := r.client.Get().Namespace(r.namespace).Resource("secrets").Do(ctx).Into(list); err != nil {
		return nil, err
	}
	return list.Items, nil
}

func (r *restSecrets) Get(ctx context.Context, name string) (*corev1.Secret, error) {
	secret := &corev1.Secret{}
	err := r.client.Get().Namespace(r.namespace).Resource("secrets").Name(name).Do(ctx).Into(secret)
	return secret, err
}

func (r *restSecrets) Create(ctx context.Context, secret *corev1.Secret) error {
	return r.client.Post().Namespace(r.namespace).Resource("secrets").Body(secret).Do(ctx).Error()
}

func (r *restSecrets) Update(ctx context.Context, secret *corev1.Secret) error {
	return r.client.Put().Namespace(r.namespace).Resource("secrets").Name(secret.Name).Body(secret).Do(ctx).Error()
}
