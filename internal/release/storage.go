package release

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"time"

	keramoserr "github.com/ebogdum/keramos/internal/errors"
	keramoslabels "github.com/ebogdum/keramos/internal/labels"
	"github.com/ebogdum/keramos/internal/logger"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
)

const (
	// The keramos-managed marker key (`managedBy`) and value (`keramos`) come
	// from internal/labels. The historical label key was "owner", which
	// collides with operator conventions where "owner" identifies the
	// human or team responsible for a release. Keramos now writes
	// "managedBy" and reads either: during a release record's lifetime
	// an upgrade rewrites the label to the new key, so old records are
	// seamlessly converted.
	labelOwner   = "owner" // legacy; read-only fallback when listing
	labelName    = "name"
	labelVersion = "version"
	labelStatus  = "status"
	dataKey      = "release"

	storageTimeout = 30 * time.Second

	// maxSecretSize is the maximum size for a Kubernetes Secret (1MB).
	maxSecretSize = 1 * 1024 * 1024
)

// Storage is the interface for persisting releases in the cluster.
type Storage interface {
	Create(rel *Release) error
	Update(rel *Release) error
	Get(name string, revision int) (*Release, error)
	List(namespace string) ([]*Release, error)
	Last(name string) (*Release, error)
	History(name string) ([]*Release, error)
	Delete(name string, revision int) error
}

// SecretStorage implements Storage using Kubernetes Secrets.
type SecretStorage struct {
	clientset kubernetes.Interface
	namespace string
}

// NewSecretStorage creates a Secrets-backed storage.
func NewSecretStorage(clientset kubernetes.Interface, namespace string) Storage {
	return &SecretStorage{
		clientset: clientset,
		namespace: namespace,
	}
}

// SecretName generates the secret name for a release revision.
func SecretName(releaseName string, revision int) string {
	return fmt.Sprintf("keramos.v1.%s.v%d", releaseName, revision)
}

// releaseRecordName matches the canonical release-record Secret name shape,
// `keramos.v1.<release>.v<revision>`.
var releaseRecordName = regexp.MustCompile(`^keramos\.v1\..+\.v\d+$`)

// isReleaseRecord reports whether a Secret is a keramos RELEASE RECORD rather than
// an ordinary application Secret that keramos merely manages. Both carry the
// `managedBy=keramos` label (keramos stamps it on every managed resource), so the
// label alone is ambiguous; a release record additionally matches the
// canonical name shape AND carries the encoded-release data key. Without this
// check, listing would try to decode arbitrary managed Secrets (e.g. an app's
// `db-password`) and emit a spurious "skipping corrupt release secret" warning.
func isReleaseRecord(secret *corev1.Secret) bool {
	if !releaseRecordName.MatchString(secret.Name) {
		return false
	}
	_, ok := secret.Data[dataKey]
	return ok
}

// SecretLabels generates the labels for a release secret. We write both
// the canonical `managedBy` and the legacy `owner` label so existing
// tooling that still selects on `owner=keramos` keeps working until it
// migrates. New deployments should select on `managedBy=keramos`.
func SecretLabels(rel *Release) map[string]string {
	return map[string]string{
		keramoslabels.ManagedByLabel: keramoslabels.ManagedByValue,
		labelOwner:     keramoslabels.ManagedByValue, // backwards-compat for owner-selecting tooling
		labelName:      rel.Name,
		labelVersion:   strconv.Itoa(rel.Revision),
		labelStatus:    string(rel.Status),
	}
}

// newStorageContext creates a context with the storage timeout.
func newStorageContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), storageTimeout)
}

func (s *SecretStorage) Create(rel *Release) error {
	encoded, err := Encode(rel)
	if nil != err {
		return err
	}

	if len(encoded) > maxSecretSize {
		return keramoserr.NewErrorf(keramoserr.ErrRelease,
			"encoded release size %d bytes exceeds K8s Secret limit of %d bytes; consider using fewer/smaller templates",
			len(encoded), maxSecretSize)
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      SecretName(rel.Name, rel.Revision),
			Namespace: s.namespace,
			Labels:    SecretLabels(rel),
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			dataKey: []byte(encoded),
		},
	}

	logger.Debug("creating release secret %s in %s", secret.Name, s.namespace)

	ctx, cancel := newStorageContext()
	defer cancel()

	_, createErr := s.clientset.CoreV1().Secrets(s.namespace).Create(
		ctx, secret, metav1.CreateOptions{},
	)
	if nil != createErr {
		return keramoserr.WrapErrorf(keramoserr.ErrRelease, createErr, "failed to create release secret for %s v%d", rel.Name, rel.Revision)
	}

	return nil
}

func (s *SecretStorage) Update(rel *Release) error {
	encoded, err := Encode(rel)
	if nil != err {
		return err
	}

	if len(encoded) > maxSecretSize {
		return keramoserr.NewErrorf(keramoserr.ErrRelease,
			"encoded release size %d bytes exceeds K8s Secret limit of %d bytes; consider using fewer/smaller templates",
			len(encoded), maxSecretSize)
	}

	secretName := SecretName(rel.Name, rel.Revision)

	ctx, cancel := newStorageContext()
	defer cancel()

	existing, getErr := s.clientset.CoreV1().Secrets(s.namespace).Get(
		ctx, secretName, metav1.GetOptions{},
	)
	if nil != getErr {
		return keramoserr.WrapErrorf(keramoserr.ErrRelease, getErr, "failed to get release secret %s", secretName)
	}

	existing.Labels = SecretLabels(rel)
	existing.Data = map[string][]byte{
		dataKey: []byte(encoded),
	}

	updateCtx, updateCancel := newStorageContext()
	defer updateCancel()

	_, updateErr := s.clientset.CoreV1().Secrets(s.namespace).Update(
		updateCtx, existing, metav1.UpdateOptions{},
	)
	if nil != updateErr {
		return keramoserr.WrapErrorf(keramoserr.ErrRelease, updateErr, "failed to update release secret %s", secretName)
	}

	return nil
}

func (s *SecretStorage) Get(name string, revision int) (*Release, error) {
	secretName := SecretName(name, revision)

	ctx, cancel := newStorageContext()
	defer cancel()

	secret, err := s.clientset.CoreV1().Secrets(s.namespace).Get(
		ctx, secretName, metav1.GetOptions{},
	)
	if nil != err {
		return nil, keramoserr.WrapErrorf(keramoserr.ErrRelease, err, "release %s revision %d not found", name, revision)
	}

	return decodeSecret(secret)
}

// List returns release secrets in the given namespace. The empty string lists
// across all namespaces; use the storage's bound namespace by passing
// AllNamespaces if you genuinely want a no-op fallback to it.
func (s *SecretStorage) List(namespace string) ([]*Release, error) {
	ctx, cancel := newStorageContext()
	defer cancel()

	// Two listings: the new managedBy label and the legacy owner label.
	// k8s label selectors AND on commas; there's no native OR, so we run
	// both queries and merge by secret name.
	seen := make(map[string]struct{})
	releases := make([]*Release, 0)
	for _, sel := range []string{
		fmt.Sprintf("%s=%s", keramoslabels.ManagedByLabel, keramoslabels.ManagedByValue),
		fmt.Sprintf("%s=%s", labelOwner, keramoslabels.ManagedByValue),
	} {
		secrets, err := s.clientset.CoreV1().Secrets(namespace).List(
			ctx, metav1.ListOptions{LabelSelector: sel},
		)
		if nil != err {
			return nil, keramoserr.WrapError(keramoserr.ErrRelease, "failed to list release secrets", err)
		}
		for i := range secrets.Items {
			if _, dup := seen[secrets.Items[i].Name]; dup {
				continue
			}
			seen[secrets.Items[i].Name] = struct{}{}
			if !isReleaseRecord(&secrets.Items[i]) {
				// A keramos-managed application Secret, not a release record.
				continue
			}
			rel, decErr := decodeSecret(&secrets.Items[i])
			if nil != decErr {
				logger.Warn("skipping corrupt release secret %s: %v", secrets.Items[i].Name, decErr)
				continue
			}
			releases = append(releases, rel)
		}
	}
	return releases, nil
}

func (s *SecretStorage) Last(name string) (*Release, error) {
	history, err := s.History(name)
	if nil != err {
		return nil, err
	}
	if 0 == len(history) {
		return nil, keramoserr.NewErrorf(keramoserr.ErrReleaseNotFound, "release %s not found", name)
	}
	return history[len(history)-1], nil
}

func (s *SecretStorage) History(name string) ([]*Release, error) {
	// Use labels.Set so any unsafe characters in `name` cannot inject
	// additional selector clauses. Apimachinery validates label values strictly.
	selector := labels.Set{labelOwner: keramoslabels.ManagedByValue, labelName: name}.String()

	ctx, cancel := newStorageContext()
	defer cancel()

	secrets, err := s.clientset.CoreV1().Secrets(s.namespace).List(
		ctx, metav1.ListOptions{LabelSelector: selector},
	)
	if nil != err {
		return nil, keramoserr.WrapErrorf(keramoserr.ErrRelease, err, "failed to list history for %s", name)
	}

	releases := make([]*Release, 0, len(secrets.Items))
	for i := range secrets.Items {
		if !isReleaseRecord(&secrets.Items[i]) {
			continue
		}
		rel, decErr := decodeSecret(&secrets.Items[i])
		if nil != decErr {
			logger.Warn("skipping corrupt release secret %s: %v", secrets.Items[i].Name, decErr)
			continue
		}
		releases = append(releases, rel)
	}

	sort.Slice(releases, func(i, j int) bool {
		return releases[i].Revision < releases[j].Revision
	})

	return releases, nil
}

func (s *SecretStorage) Delete(name string, revision int) error {
	secretName := SecretName(name, revision)

	ctx, cancel := newStorageContext()
	defer cancel()

	err := s.clientset.CoreV1().Secrets(s.namespace).Delete(
		ctx, secretName, metav1.DeleteOptions{},
	)
	if nil != err {
		if k8serrors.IsNotFound(err) {
			return nil
		}
		return keramoserr.WrapErrorf(keramoserr.ErrRelease, err, "failed to delete release secret %s", secretName)
	}
	return nil
}

func decodeSecret(secret *corev1.Secret) (*Release, error) {
	data, exists := secret.Data[dataKey]
	if !exists {
		return nil, keramoserr.NewErrorf(keramoserr.ErrRelease, "release secret %s missing data key", secret.Name)
	}
	return Decode(string(data))
}
