package clients

import (
	"bytes"
	"encoding/json"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/static"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/kuberik/propagation-controller/api/v1alpha1"
	"github.com/kuberik/propagation-controller/pkg/oci"
)

type PropagationBackendOCIClientFactory func(repository name.Repository, ociClient []crane.Option) PropagationBackendOCIClient

type PropagationBackendOCIClient struct {
	name.Repository
	oci.OCIClient
	DeploymentStatusesCache map[string]v1.Image
	CurrentStatus           v1alpha1.DeploymentStatus
}

func NewPropagationBackendOCIClient(repository name.Repository, client oci.OCIClient) PropagationBackendOCIClient {
	return PropagationBackendOCIClient{
		Repository:              repository,
		OCIClient:               client,
		DeploymentStatusesCache: make(map[string]v1.Image),
	}

}

func newStatusImage(status v1alpha1.DeploymentStatus) (v1.Image, error) {
	statusesJSON, err := json.Marshal(status)
	if err != nil {
		return nil, err
	}

	layer := static.NewLayer(statusesJSON, types.MediaType("application/json"))
	image, err := mutate.AppendLayers(empty.Image, layer)
	if err != nil {
		return nil, err
	}
	return image, nil
}

const (
	statusesSubPath  = "statuses"
	manifestsSubPath = "manifests"
	deploySubPath    = "deploy"
)

func (c *PropagationBackendOCIClient) statusTag(deployment string) string {
	return c.Repository.Registry.Repo(c.Repository.RepositoryStr(), statusesSubPath, deployment).Tag(name.DefaultTag).Name()
}

func (c *PropagationBackendOCIClient) PublishStatus(deployment string, status v1alpha1.DeploymentStatus) error {
	if status == c.CurrentStatus {
		return nil
	}
	image, err := newStatusImage(status)
	if err != nil {
		return err
	}

	if err := c.OCIClient.Push(image, c.statusTag(deployment)); err != nil {
		return err
	}
	c.CurrentStatus = status
	return nil
}

func extractStatus(image v1.Image) (*v1alpha1.DeploymentStatus, error) {
	var extracted bytes.Buffer
	err := crane.Export(image, &extracted)
	if err != nil {
		return nil, err
	}

	result := &v1alpha1.DeploymentStatus{}
	err = json.Unmarshal(extracted.Bytes(), result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *PropagationBackendOCIClient) GetStatus(deployment string) (*v1alpha1.DeploymentStatus, error) {
	statusTag := c.statusTag(deployment)
	if cachedStatus, ok := c.DeploymentStatusesCache[deployment]; ok {
		cachedDigest, digestErr := cachedStatus.Digest()
		remoteDigest, err := c.OCIClient.Digest(statusTag)
		if err == nil && digestErr == nil && remoteDigest == cachedDigest.String() {
			return extractStatus(cachedStatus)
		}
	}

	image, err := c.OCIClient.Pull(statusTag)
	if err != nil {
		return nil, err
	}

	status, err := extractStatus(image)
	if err != nil {
		return nil, err
	}

	c.DeploymentStatusesCache[deployment] = image
	return status, nil
}

func (c *PropagationBackendOCIClient) Propagate(deployment, version string) error {
	image, err := c.OCIClient.Pull(
		c.Repository.Registry.Repo(c.Repository.RepositoryStr(), manifestsSubPath, deployment).Tag(version).Name(),
	)
	if err != nil {
		return err
	}

	if err := c.OCIClient.Push(
		image,
		c.Repository.Registry.Repo(c.Repository.RepositoryStr(), deploySubPath, deployment).Tag(name.DefaultTag).Name(),
	); err != nil {
		return err
	}
	return nil
}
