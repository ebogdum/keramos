package release

import (
	"fmt"
	"sort"
	"strconv"

	keramoserr "github.com/ebogdum/keramos/v3/internal/errors"
	keramoslabels "github.com/ebogdum/keramos/v3/internal/labels"
	"github.com/ebogdum/keramos/v3/internal/logger"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
)

// ConfigMapStorage persists releases in Kubernetes ConfigMaps. Mirrors the
// SecretStorage layout but uses CoreV1().ConfigMaps for backends that prefer
// non-Secret storage (e.g. when Secret access is restricted by RBAC).
type ConfigMapStorage struct {
	clientset kubernetes.Interface
	namespace string
}

// NewConfigMapStorage creates a ConfigMaps-backed storage.
func NewConfigMapStorage(clientset kubernetes.Interface, namespace string) Storage {
	return &ConfigMapStorage{clientset: clientset, namespace: namespace}
}

// ConfigMapName generates the configmap name for a release revision.
func ConfigMapName(releaseName string, revision int) string {
	return fmt.Sprintf("keramos.v1.%s.v%d", releaseName, revision)
}

func (s *ConfigMapStorage) Create(rel *Release) error {
	encoded, err := Encode(rel)
	if nil != err {
		return err
	}
	if int64(len(encoded)) > maxSecretSize {
		return keramoserr.NewErrorf(keramoserr.ErrRelease,
			"release payload exceeds storage size limit of %d bytes", maxSecretSize)
	}

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ConfigMapName(rel.Name, rel.Revision),
			Namespace: s.namespace,
			Labels: map[string]string{
				keramoslabels.ManagedByLabel: keramoslabels.ManagedByValue,
				labelOwner:     keramoslabels.ManagedByValue, // backwards-compat with legacy label
				labelName:      rel.Name,
				labelVersion:   strconv.Itoa(rel.Revision),
				labelStatus:    string(rel.Status),
			},
		},
		Data: map[string]string{dataKey: encoded},
	}

	logger.Debug("creating release configmap %s in %s", cm.Name, s.namespace)
	ctx, cancel := newStorageContext()
	defer cancel()
	_, createErr := s.clientset.CoreV1().ConfigMaps(s.namespace).Create(ctx, cm, metav1.CreateOptions{})
	if nil != createErr {
		return keramoserr.WrapErrorf(keramoserr.ErrRelease, createErr, "failed to create release configmap %s", cm.Name)
	}
	return nil
}

func (s *ConfigMapStorage) Update(rel *Release) error {
	encoded, err := Encode(rel)
	if nil != err {
		return err
	}
	if int64(len(encoded)) > maxSecretSize {
		return keramoserr.NewErrorf(keramoserr.ErrRelease,
			"release payload exceeds storage size limit of %d bytes", maxSecretSize)
	}

	name := ConfigMapName(rel.Name, rel.Revision)
	ctx, cancel := newStorageContext()
	defer cancel()

	existing, getErr := s.clientset.CoreV1().ConfigMaps(s.namespace).Get(ctx, name, metav1.GetOptions{})
	if nil != getErr {
		return keramoserr.WrapErrorf(keramoserr.ErrRelease, getErr, "failed to fetch release configmap %s for update", name)
	}
	existing.Data = map[string]string{dataKey: encoded}
	if nil == existing.Labels {
		existing.Labels = map[string]string{}
	}
	existing.Labels[labelStatus] = string(rel.Status)

	_, updateErr := s.clientset.CoreV1().ConfigMaps(s.namespace).Update(ctx, existing, metav1.UpdateOptions{})
	if nil != updateErr {
		return keramoserr.WrapErrorf(keramoserr.ErrRelease, updateErr, "failed to update release configmap %s", name)
	}
	return nil
}

func (s *ConfigMapStorage) Get(name string, revision int) (*Release, error) {
	ctx, cancel := newStorageContext()
	defer cancel()

	cm, err := s.clientset.CoreV1().ConfigMaps(s.namespace).Get(ctx, ConfigMapName(name, revision), metav1.GetOptions{})
	if nil != err {
		if k8serrors.IsNotFound(err) {
			return nil, keramoserr.NewErrorf(keramoserr.ErrReleaseNotFound, "release %s revision %d not found", name, revision)
		}
		return nil, keramoserr.WrapError(keramoserr.ErrRelease, "failed to get release configmap", err)
	}
	return decodeConfigMap(cm)
}

func (s *ConfigMapStorage) List(namespace string) ([]*Release, error) {
	selector := fmt.Sprintf("%s=%s", labelOwner, keramoslabels.ManagedByValue)
	ctx, cancel := newStorageContext()
	defer cancel()

	cms, err := s.clientset.CoreV1().ConfigMaps(namespace).List(ctx, metav1.ListOptions{LabelSelector: selector})
	if nil != err {
		return nil, keramoserr.WrapError(keramoserr.ErrRelease, "failed to list release configmaps", err)
	}
	out := make([]*Release, 0, len(cms.Items))
	for i := range cms.Items {
		rel, decErr := decodeConfigMap(&cms.Items[i])
		if nil != decErr {
			logger.Warn("skipping corrupt release configmap %s: %v", cms.Items[i].Name, decErr)
			continue
		}
		out = append(out, rel)
	}
	return out, nil
}

func (s *ConfigMapStorage) Last(name string) (*Release, error) {
	history, err := s.History(name)
	if nil != err {
		return nil, err
	}
	if 0 == len(history) {
		return nil, keramoserr.NewErrorf(keramoserr.ErrReleaseNotFound, "release %s not found", name)
	}
	return history[len(history)-1], nil
}

func (s *ConfigMapStorage) History(name string) ([]*Release, error) {
	selector := labels.Set{labelOwner: keramoslabels.ManagedByValue, labelName: name}.String()
	ctx, cancel := newStorageContext()
	defer cancel()

	cms, err := s.clientset.CoreV1().ConfigMaps(s.namespace).List(ctx, metav1.ListOptions{LabelSelector: selector})
	if nil != err {
		return nil, keramoserr.WrapErrorf(keramoserr.ErrRelease, err, "failed to list history for %s", name)
	}
	out := make([]*Release, 0, len(cms.Items))
	for i := range cms.Items {
		rel, decErr := decodeConfigMap(&cms.Items[i])
		if nil != decErr {
			logger.Warn("skipping corrupt release configmap %s: %v", cms.Items[i].Name, decErr)
			continue
		}
		out = append(out, rel)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Revision < out[j].Revision })
	return out, nil
}

func (s *ConfigMapStorage) Delete(name string, revision int) error {
	ctx, cancel := newStorageContext()
	defer cancel()
	if err := s.clientset.CoreV1().ConfigMaps(s.namespace).Delete(ctx, ConfigMapName(name, revision), metav1.DeleteOptions{}); nil != err {
		if k8serrors.IsNotFound(err) {
			return nil
		}
		return keramoserr.WrapErrorf(keramoserr.ErrRelease, err, "failed to delete release configmap")
	}
	return nil
}

func decodeConfigMap(cm *corev1.ConfigMap) (*Release, error) {
	encoded, ok := cm.Data[dataKey]
	if !ok {
		return nil, keramoserr.NewError(keramoserr.ErrRelease, "configmap missing release data key")
	}
	return Decode(encoded)
}
