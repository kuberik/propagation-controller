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
	statusesJSON, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	layer := static.NewLayer(statusesJSON, types.MediaType("application/json"))
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

	if err := json.Unmarshal(artifactData, dest); err != nil {
		return err
	}
	return nil
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

func (pc *PropagationClientset) Propagation(propagation v1alpha1.Propagation) (*PropagationClient, *PropagationConfigClient, error) {
	secret := &corev1.Secret{}
	if propagation.Spec.Backend.SecretRef != nil && propagation.Spec.Backend.SecretRef.Name != "" {
		err := pc.k8sClient.Get(context.TODO(), k8stypes.NamespacedName{
			Name:      propagation.Spec.Backend.SecretRef.Name,
			Namespace: propagation.Namespace,
		}, secret)
		if err != nil {
			return nil, nil, err
		}
	}

	key := k8stypes.NamespacedName{Name: propagation.Name, Namespace: propagation.Namespace}
	newBackendClient, err := newPropagationBackendClient(propagation.Spec.Backend, secret.Data)
	if err != nil {
		return nil, nil, err
	}
	configClient := NewPropagationConfigClient(newBackendClient)
	if c, ok := pc.clients[key]; ok && reflect.DeepEqual(c.client, newBackendClient) {
		return c, &configClient, nil
	}
	client := NewPropagationClient(newBackendClient)
	pc.clients[key] = &client
	return &client, &configClient, nil
}

type PropagationClient struct {
	client PropagationBackendClient
	cache  propagationClientCache
}

type propagationClientCache struct {
	deploymentStatuses map[string]Artifact
	publishedStatus    v1alpha1.DeploymentStatus
	propagatedVersion  string
}

func NewPropagationClient(client PropagationBackendClient) PropagationClient {
	return PropagationClient{
		client: client,
		cache: propagationClientCache{
			deploymentStatuses: make(map[string]Artifact),
		},
	}
}

func (c *PropagationClient) PublishStatus(deployment string, status v1alpha1.DeploymentStatus) error {
	if status == c.cache.publishedStatus {
		return nil
	}
	artifact, err := c.client.NewArtifact(status)
	if err != nil {
		return err
	}

	if err := c.client.Publish(ArtifactMetadata{Deployment: deployment, Type: DeployStatusArtifactType}, artifact); err != nil {
		return err
	}
	c.cache.publishedStatus = status
	return nil
}

func (c *PropagationClient) GetStatus(deployment string) (*v1alpha1.DeploymentStatus, error) {
	artifactMetadata := ArtifactMetadata{Type: DeployStatusArtifactType, Deployment: deployment}
	var artifact Artifact
	if cachedStatus, ok := c.cache.deploymentStatuses[deployment]; ok {
		cachedDigest, digestErr := cachedStatus.DigestString()
		remoteDigest, err := c.client.Digest(artifactMetadata)
		if err == nil && digestErr == nil && remoteDigest == cachedDigest {
			artifact = cachedStatus
		}
	}

	var err error
	if artifact == nil {
		artifact, err = c.client.Fetch(artifactMetadata)
		if err != nil {
			return nil, err
		}
	}

	status := &v1alpha1.DeploymentStatus{}
	if err := c.client.ParseArtifact(artifact, status); err != nil {
		return nil, fmt.Errorf("failed to parse deployment status: %w", err)
	}

	c.cache.deploymentStatuses[deployment] = artifact
	return status, nil
}

func (c *PropagationClient) Propagate(deployment, version string) error {
	if version == c.cache.propagatedVersion {
		return nil
	}
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
	c.cache.propagatedVersion = version
	return nil
}

type PropagationConfigClient struct {
	client PropagationBackendClient
}

func NewPropagationConfigClient(client PropagationBackendClient) PropagationConfigClient {
	return PropagationConfigClient{
		client: client,
	}
}

func (c *PropagationConfigClient) PublishConfig(config config.Config) error {
	artifact, err := c.client.NewArtifact(config)
	if err != nil {
		return err
	}
	if err := c.client.Publish(
		ArtifactMetadata{Type: PropagationConfigArtifactType},
		artifact,
	); err != nil {
		return err
	}
	return nil
}

func (c *PropagationConfigClient) GetConfig() (*config.Config, error) {
	artifact, err := c.client.Fetch(ArtifactMetadata{Type: PropagationConfigArtifactType})
	if err != nil {
		return nil, err
	}

	config := &config.Config{}
	if err := c.client.ParseArtifact(artifact, config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}
	return config, nil
}
