package clients

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/static"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/kuberik/propagation-controller/api/v1alpha1"
	"github.com/kuberik/propagation-controller/pkg/repo/config"
	corev1 "k8s.io/api/core/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Artifact interface {
	DigestString() (string, error)
	Bytes() ([]byte, error)
}

type ArtifactType int

const (
	DeployStatusArtifactType ArtifactType = iota
	ManifestArtifactType
	DeployArtifactType
	PropagationConfigArtifactType
)

type ArtifactMetadata struct {
	Deployment string
	Type       ArtifactType
	Version    string
}

type PropagationBackendClient interface {
	Fetch(ArtifactMetadata) (Artifact, error)
	Digest(ArtifactMetadata) (string, error)
	Publish(ArtifactMetadata, Artifact) error
	NewArtifact(data any) (Artifact, error)
	ParseArtifact(a Artifact, dest any) error
}

var _ Artifact = &ociArtifact{}

type ociArtifact struct {
	image v1.Image
	data  []byte
}

// DigestString implements Artifact.
func (a *ociArtifact) DigestString() (string, error) {
	digest, err := a.image.Digest()
	return digest.String(), err
}

// Bytes implements Artifact.
func (a *ociArtifact) Bytes() ([]byte, error) {
	if a.data != nil {
		return a.data, nil
	}
	var extracted bytes.Buffer
	err := crane.Export(a.image, &extracted)
	if err != nil {
		return nil, err
	}
	a.data = extracted.Bytes()
	return a.data, nil
}

var _ PropagationBackendClient = &OCIPropagationBackendClient{}

type OCIPropagationBackendClient struct {
	repository name.Repository
	auth       *authn.AuthConfig
}

func NewOCIPropagationBackendClient(repository name.Repository) OCIPropagationBackendClient {
	return OCIPropagationBackendClient{
		repository: repository,
	}
}

func (c *OCIPropagationBackendClient) ociTagFromArtifactMetadata(m ArtifactMetadata) string {
	var subpath string
	version := m.Version
	switch m.Type {
	case DeployStatusArtifactType:
		subpath = "statuses"
		version = name.DefaultTag
	case ManifestArtifactType:
		subpath = "manifests"
	case DeployArtifactType:
		version = name.DefaultTag
		subpath = "deploy"
	case PropagationConfigArtifactType:
		version = name.DefaultTag
		subpath = "config"
	default:
		panic("unknown artifact type")
	}
	return c.repository.Registry.Repo(c.repository.RepositoryStr(), subpath, m.Deployment).Tag(version).Name()
}

// Digest implements PropagationBackendClient.
func (c *OCIPropagationBackendClient) Digest(m ArtifactMetadata) (string, error) {
	return crane.Digest(c.ociTagFromArtifactMetadata(m), c.options()...)
}

// Fetch implements PropagationBackendClient.
func (c *OCIPropagationBackendClient) Fetch(m ArtifactMetadata) (Artifact, error) {
	image, err := crane.Pull(c.ociTagFromArtifactMetadata(m), c.options()...)
	if err != nil {
		return nil, err
	}
	return &ociArtifact{image: image}, err
}

// Publish implements PropagationBackendClient.
func (c *OCIPropagationBackendClient) Publish(m ArtifactMetadata, a Artifact) error {
	if ociArtifact, ok := a.(*ociArtifact); ok {
		return crane.Push(ociArtifact.image, c.ociTagFromArtifactMetadata(m), c.options()...)
	}
	return fmt.Errorf("incompatible artifact for OCI client")
}

// NewArtifact implements PropagationBackendClient.
func (*OCIPropagationBackendClient) NewArtifact(data any) (Artifact, error) {
	artifactJSON, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	layer := static.NewLayer(artifactJSON, types.MediaType("application/json"))
	image, err := mutate.AppendLayers(empty.Image, layer)
	if err != nil {
		return nil, err
	}
	return &ociArtifact{image: image}, nil
}

// ParseArtifact implements PropagationBackendClient.
func (c *OCIPropagationBackendClient) ParseArtifact(a Artifact, dest any) error {
	artifactData, err := a.Bytes()
	if err != nil {
		return err
	}

	return json.Unmarshal(artifactData, dest)
}

func (c *OCIPropagationBackendClient) options() []crane.Option {
	options := []crane.Option{}
	if c.auth != nil {
		options = append(options, crane.WithAuth(authn.FromConfig(*c.auth)))
	}
	return options
}

type PropagationClientset struct {
	k8sClient client.Client
	clients   map[k8stypes.NamespacedName]*PropagationClient
}

func newPropagationBackendClient(baseUrl v1alpha1.PropagationBackend, secretData map[string][]byte) (PropagationBackendClient, error) {
	protocol, err := baseUrl.Scheme()
	if err != nil {
		return nil, err
	}
	url, err := baseUrl.TrimScheme()
	if err != nil {
		return nil, err
	}

	switch protocol {
	case "oci":
		repository, err := name.NewRepository(url)
		if err != nil {
			return nil, fmt.Errorf("failed to parse OCI repository: %w", err)
		}

		authConfig := &authn.AuthConfig{}
		if secretData[corev1.DockerConfigJsonKey] != nil {
			err = json.Unmarshal(secretData[corev1.DockerConfigJsonKey], authConfig)
			if err != nil {
				return nil, fmt.Errorf("failed to parse docker auth config: %w", err)
			}
		} else {
			authConfig = nil
		}

		return &OCIPropagationBackendClient{
			repository: repository,
			auth:       authConfig,
		}, nil
	default:
		return nil, fmt.Errorf("%s backend not supported", protocol)
	}
}

func NewPropagationClientset(k8sClient client.Client) PropagationClientset {
	clientset := PropagationClientset{
		clients:   make(map[k8stypes.NamespacedName]*PropagationClient),
		k8sClient: k8sClient,
	}
	return clientset
}

func (pc *PropagationClientset) Propagation(propagation v1alpha1.Propagation) (*PropagationClient, error) {
	secret := &corev1.Secret{}
	if propagation.Spec.Backend.SecretRef != nil && propagation.Spec.Backend.SecretRef.Name != "" {
		err := pc.k8sClient.Get(context.TODO(), k8stypes.NamespacedName{
			Name:      propagation.Spec.Backend.SecretRef.Name,
			Namespace: propagation.Namespace,
		}, secret)
		if err != nil {
			return nil, err
		}
	}

	key := k8stypes.NamespacedName{Name: propagation.Name, Namespace: propagation.Namespace}
	newBackendClient, err := newPropagationBackendClient(propagation.Spec.Backend, secret.Data)
	if err != nil {
		return nil, err
	}

	var client *PropagationClient
	if c, ok := pc.clients[key]; ok {
		client = c
		client.client.PropagationBackendClient = newBackendClient
	} else {
		c := NewPropagationClient(newBackendClient)
		client = &c
	}
	pc.clients[key] = client
	return client, nil
}

var _ PropagationBackendClient = &CachedPropagationBackendClient{}

type CachedPropagationBackendClient struct {
	PropagationBackendClient
	fetchCache   map[ArtifactMetadata]Artifact
	publishCache map[ArtifactMetadata]Artifact
}

func NewCachedPropagationBackendClient(client PropagationBackendClient) CachedPropagationBackendClient {
	return CachedPropagationBackendClient{
		PropagationBackendClient: client,
		fetchCache:               make(map[ArtifactMetadata]Artifact),
		publishCache:             make(map[ArtifactMetadata]Artifact),
	}
}

// Fetch implements PropagationBackendClient.
func (c *CachedPropagationBackendClient) Fetch(metadata ArtifactMetadata) (Artifact, error) {
	if cached, ok := c.fetchCache[metadata]; ok {
		// Manifest artifact should be immutable
		if metadata.Type == ManifestArtifactType {
			return cached, nil
		}
		cachedDigest, digestErr := cached.DigestString()
		remoteDigest, err := c.Digest(metadata)
		if err == nil && digestErr == nil && remoteDigest == cachedDigest {
			return cached, nil
		}
	}
	a, err := c.PropagationBackendClient.Fetch(metadata)
	if err != nil {
		return nil, err
	}
	c.fetchCache[metadata] = a
	return a, nil
}

// Publish implements PropagationBackendClient.
func (c *CachedPropagationBackendClient) Publish(metadata ArtifactMetadata, a Artifact) error {
	if cached, ok := c.publishCache[metadata]; ok {
		cachedBytes, err := cached.Bytes()
		if err != nil {
			return err
		}
		publishBytes, err := a.Bytes()
		if err != nil {
			return err
		}
		if reflect.DeepEqual(cachedBytes, publishBytes) {
			return nil
		}
	}
	err := c.PropagationBackendClient.Publish(metadata, a)
	if err != nil {
		return err
	}
	c.publishCache[metadata] = a
	return nil
}

type PropagationClient struct {
	client CachedPropagationBackendClient
}

func NewPropagationClient(client PropagationBackendClient) PropagationClient {
	return PropagationClient{
		client: NewCachedPropagationBackendClient(client),
	}
}

func (c *PropagationClient) publishArtifact(metadata ArtifactMetadata, data interface{}) error {
	artifact, err := c.client.NewArtifact(data)
	if err != nil {
		return err
	}
	return c.client.Publish(metadata, artifact)
}

func (c *PropagationClient) PublishStatus(deployment string, status v1alpha1.DeploymentStatus) error {
	return c.publishArtifact(ArtifactMetadata{Deployment: deployment, Type: DeployStatusArtifactType}, status)
}

func (c *PropagationClient) fetchArtifact(metadata ArtifactMetadata, dest interface{}) error {
	artifact, err := c.client.Fetch(metadata)
	if err != nil {
		return err
	}
	return c.client.ParseArtifact(artifact, dest)
}

func (c *PropagationClient) GetStatus(deployment string) (*v1alpha1.DeploymentStatus, error) {
	status := &v1alpha1.DeploymentStatus{}
	err := c.fetchArtifact(ArtifactMetadata{Type: DeployStatusArtifactType, Deployment: deployment}, status)
	if err != nil {
		return nil, err
	}
	return status, nil
}

func (c *PropagationClient) Propagate(deployment, version string) error {
	artifact, err := c.client.Fetch(
		ArtifactMetadata{Type: ManifestArtifactType, Deployment: deployment, Version: version},
	)
	if err != nil {
		return err
	}

	if err := c.client.Publish(
		ArtifactMetadata{Type: DeployArtifactType, Deployment: deployment},
		artifact,
	); err != nil {
		return err
	}
	return nil
}

func (c *PropagationClient) PublishConfig(config config.Config) error {
	return c.publishArtifact(
		ArtifactMetadata{Type: PropagationConfigArtifactType},
		config,
	)
}

func (c *PropagationClient) GetConfig() (*config.Config, error) {
	config := &config.Config{}
	err := c.fetchArtifact(ArtifactMetadata{Type: PropagationConfigArtifactType}, config)
	if err != nil {
		return nil, err
	}
	return config, nil
}
