package promotion

import (
	"net/url"
	"strings"
)

// import (
// 	"bytes"
// 	"context"
// 	"encoding/json"
// 	"fmt"
// 	"net/url"
// 	"reflect"
// 	"strings"

// 	"github.com/google/go-containerregistry/pkg/authn"
// 	"github.com/google/go-containerregistry/pkg/crane"
// 	"github.com/google/go-containerregistry/pkg/name"
// 	v1 "github.com/google/go-containerregistry/pkg/v1"
// 	"github.com/kuberik/propagation-controller/api/promotion/v1alpha1"
// 	corev1 "k8s.io/api/core/v1"
// 	k8stypes "k8s.io/apimachinery/pkg/types"
// 	"sigs.k8s.io/controller-runtime/pkg/client"
// )

// type Artifact interface {
// 	DigestString() (string, error)
// 	Bytes() ([]byte, error)
// }

// type ArtifactType int

// const (
// 	DeployStatusArtifactType ArtifactType = iota
// 	ManifestArtifactType
// 	DeployArtifactType
// 	PromotionConfigArtifactType
// )

// type ArtifactMetadata struct {
// 	Repository string
// 	Version    string
// }

// type PromotionBackendClient interface {
// 	Fetch(ArtifactMetadata) (Artifact, error)
// 	Digest(ArtifactMetadata) (string, error)
// 	Publish(ArtifactMetadata, Artifact) error
// }

// var _ Artifact = &ociArtifact{}

// type ociArtifact struct {
// 	image v1.Image
// 	data  []byte
// }

// // DigestString implements Artifact.
// func (a *ociArtifact) DigestString() (string, error) {
// 	digest, err := a.image.Digest()
// 	return digest.String(), err
// }

// // Bytes implements Artifact.
// func (a *ociArtifact) Bytes() ([]byte, error) {
// 	if a.data != nil {
// 		return a.data, nil
// 	}
// 	var extracted bytes.Buffer
// 	err := crane.Export(a.image, &extracted)
// 	if err != nil {
// 		return nil, err
// 	}
// 	a.data = extracted.Bytes()
// 	return a.data, nil
// }

// var _ PromotionBackendClient = &OCIPromotionBackendClient{}

// type OCIPromotionBackendClient struct {
// 	repository name.Repository
// 	auth       *authn.AuthConfig
// }

// func NewOCIPromotionBackendClient(repository name.Repository) OCIPromotionBackendClient {
// 	return OCIPromotionBackendClient{
// 		repository: repository,
// 	}
// }

// func (c *OCIPromotionBackendClient) ociTagFromArtifactMetadata(m ArtifactMetadata) string {
// 	var subpath string
// 	version := m.Version
// 	switch m.Type {
// 	case DeployStatusArtifactType:
// 		subpath = "statuses"
// 		version = name.DefaultTag
// 	case ManifestArtifactType:
// 		subpath = "manifests"
// 	case DeployArtifactType:
// 		version = name.DefaultTag
// 		subpath = "deploy"
// 	case PromotionConfigArtifactType:
// 		version = name.DefaultTag
// 		subpath = "config"
// 	default:
// 		panic("unknown artifact type")
// 	}
// 	return c.repository.Registry.Repo(c.repository.RepositoryStr(), subpath, m.Deployment).Tag(version).Name()
// }

// // Digest implements PromotionBackendClient.
// func (c *OCIPromotionBackendClient) Digest(m ArtifactMetadata) (string, error) {
// 	return crane.Digest(c.ociTagFromArtifactMetadata(m), c.options()...)
// }

// // Fetch implements PromotionBackendClient.
// func (c *OCIPromotionBackendClient) Fetch(m ArtifactMetadata) (Artifact, error) {
// 	image, err := crane.Pull(c.ociTagFromArtifactMetadata(m), c.options()...)
// 	if err != nil {
// 		return nil, err
// 	}
// 	return &ociArtifact{image: image}, err
// }

// // Publish implements PromotionBackendClient.
// func (c *OCIPromotionBackendClient) Publish(m ArtifactMetadata, a Artifact) error {
// 	if ociArtifact, ok := a.(*ociArtifact); ok {
// 		return crane.Push(ociArtifact.image, c.ociTagFromArtifactMetadata(m), c.options()...)
// 	}
// 	return fmt.Errorf("incompatible artifact for OCI client")
// }

// func (c *OCIPromotionBackendClient) options() []crane.Option {
// 	options := []crane.Option{}
// 	if c.auth != nil {
// 		options = append(options, crane.WithAuth(authn.FromConfig(*c.auth)))
// 	}
// 	return options
// }

// type PromotionClientset struct {
// 	k8sClient client.Client
// 	clients   map[k8stypes.NamespacedName]*PromotionClient
// }

func scheme(baseUrl string) (string, error) {
	u, err := url.Parse(string(baseUrl))
	if err != nil {
		return "", err
	}
	return u.Scheme, nil
}

func trimScheme(baseUrl string) (string, error) {
	scheme, err := scheme(baseUrl)
	if err != nil {
		return "", err
	}
	return strings.TrimPrefix(string(baseUrl), scheme+"://"), nil
}

// func NewPromotionBackendClient(baseUrl string) (PromotionBackendClient, error) {
// 	return newPromotionBackendClient(baseUrl, nil)
// }

// func newPromotionBackendClient(baseUrl string, secretData map[string][]byte) (PromotionBackendClient, error) {
// 	protocol, err := scheme(baseUrl)
// 	if err != nil {
// 		return nil, err
// 	}
// 	url, err := trimScheme(baseUrl)
// 	if err != nil {
// 		return nil, err
// 	}

// 	switch protocol {
// 	case "oci":
// 		repository, err := name.NewRepository(url)
// 		if err != nil {
// 			return nil, fmt.Errorf("failed to parse OCI repository: %w", err)
// 		}

// 		authConfig := &authn.AuthConfig{}
// 		if secretData[corev1.DockerConfigJsonKey] != nil {
// 			err = json.Unmarshal(secretData[corev1.DockerConfigJsonKey], authConfig)
// 			if err != nil {
// 				return nil, fmt.Errorf("failed to parse docker auth config: %w", err)
// 			}
// 		} else {
// 			authConfig = nil
// 		}

// 		return &OCIPromotionBackendClient{
// 			repository: repository,
// 			auth:       authConfig,
// 		}, nil
// 	default:
// 		return nil, fmt.Errorf("%s backend not supported", protocol)
// 	}
// }

// func NewPromotionClientset(k8sClient client.Client) PromotionClientset {
// 	clientset := PromotionClientset{
// 		clients:   make(map[k8stypes.NamespacedName]*PromotionClient),
// 		k8sClient: k8sClient,
// 	}
// 	return clientset
// }

// func (pc *PromotionClientset) Promotion(promotion v1alpha1.Promotion) (*PromotionClient, error) {
// 	secret := &corev1.Secret{}
// 	if promotion.Spec.Backend.SecretRef != nil && promotion.Spec.Backend.SecretRef.Name != "" {
// 		err := pc.k8sClient.Get(context.TODO(), k8stypes.NamespacedName{
// 			Name:      promotion.Spec.Backend.SecretRef.Name,
// 			Namespace: promotion.Namespace,
// 		}, secret)
// 		if err != nil {
// 			return nil, err
// 		}
// 	}

// 	key := k8stypes.NamespacedName{Name: promotion.Name, Namespace: promotion.Namespace}
// 	newBackendClient, err := newPromotionBackendClient(promotion.Spec.Backend.BaseUrl, secret.Data)
// 	if err != nil {
// 		return nil, err
// 	}

// 	var client *PromotionClient
// 	if c, ok := pc.clients[key]; ok {
// 		client = c
// 		client.client.PromotionBackendClient = newBackendClient
// 	} else {
// 		c := NewPromotionClient(newBackendClient)
// 		client = &c
// 	}
// 	pc.clients[key] = client
// 	return client, nil
// }

// var _ PromotionBackendClient = &CachedPromotionBackendClient{}

// type CachedPromotionBackendClient struct {
// 	PromotionBackendClient
// 	fetchCache   map[ArtifactMetadata]Artifact
// 	publishCache map[ArtifactMetadata]Artifact
// }

// func NewCachedPromotionBackendClient(client PromotionBackendClient) CachedPromotionBackendClient {
// 	return CachedPromotionBackendClient{
// 		PromotionBackendClient: client,
// 		fetchCache:             make(map[ArtifactMetadata]Artifact),
// 		publishCache:           make(map[ArtifactMetadata]Artifact),
// 	}
// }

// // Fetch implements PromotionBackendClient.
// func (c *CachedPromotionBackendClient) Fetch(metadata ArtifactMetadata) (Artifact, error) {
// 	if cached, ok := c.fetchCache[metadata]; ok {
// 		// Manifest artifact should be immutable
// 		if metadata.Type == ManifestArtifactType && metadata.Version != name.DefaultTag {
// 			return cached, nil
// 		}
// 		cachedDigest, digestErr := cached.DigestString()
// 		remoteDigest, err := c.Digest(metadata)
// 		if err == nil && digestErr == nil && remoteDigest == cachedDigest {
// 			return cached, nil
// 		}
// 	}
// 	a, err := c.PromotionBackendClient.Fetch(metadata)
// 	if err != nil {
// 		return nil, err
// 	}
// 	c.fetchCache[metadata] = a
// 	return a, nil
// }

// // Publish implements PromotionBackendClient.
// func (c *CachedPromotionBackendClient) Publish(metadata ArtifactMetadata, a Artifact) error {
// 	if cached, ok := c.publishCache[metadata]; ok {
// 		cachedBytes, err := cached.Bytes()
// 		if err != nil {
// 			return err
// 		}
// 		publishBytes, err := a.Bytes()
// 		if err != nil {
// 			return err
// 		}
// 		if reflect.DeepEqual(cachedBytes, publishBytes) {
// 			return nil
// 		}
// 	}
// 	err := c.PromotionBackendClient.Publish(metadata, a)
// 	if err != nil {
// 		return err
// 	}
// 	c.publishCache[metadata] = a
// 	return nil
// }

// type PromotionClient struct {
// 	client CachedPromotionBackendClient
// }

// func NewPromotionClient(client PromotionBackendClient) PromotionClient {
// 	return PromotionClient{
// 		client: NewCachedPromotionBackendClient(client),
// 	}
// }

// func (c *PromotionClient) publishArtifact(metadata ArtifactMetadata, data interface{}) error {
// 	artifact, err := c.client.NewArtifact(data)
// 	if err != nil {
// 		return err
// 	}
// 	return c.client.Publish(metadata, artifact)
// }

// func (c *PromotionClient) fetchArtifact(metadata ArtifactMetadata, dest interface{}) error {
// 	artifact, err := c.client.Fetch(metadata)
// 	if err != nil {
// 		return err
// 	}
// 	return c.client.ParseArtifact(artifact, dest)
// }

// func (c *PromotionClient) Promote(deployment, version string) error {
// 	artifact, err := c.client.Fetch(
// 		ArtifactMetadata{Type: ManifestArtifactType, Deployment: deployment, Version: version},
// 	)
// 	if err != nil {
// 		return err
// 	}

// 	if err := c.client.Publish(
// 		ArtifactMetadata{Type: DeployArtifactType, Deployment: deployment},
// 		artifact,
// 	); err != nil {
// 		return err
// 	}
// 	return nil
// }
