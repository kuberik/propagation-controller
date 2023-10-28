package status

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

type StatusClient struct {
	name.Repository
	oci.OCIClient
}

func NewStatusClient(repository string, ociClient oci.OCIClient) (*StatusClient, error) {
	repositoryParsed, err := name.NewRepository(repository)
	if err != nil {
		return nil, err
	}

	return &StatusClient{
		Repository: repositoryParsed,
		OCIClient:  ociClient,
	}, nil
}

func newStatusesImage(statuses []v1alpha1.DeploymentStatus) (v1.Image, error) {
	statusesJSON, err := json.Marshal(statuses)
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

func (c *StatusClient) PublishStatuses(deployment string, statuses []v1alpha1.DeploymentStatus) error {
	image, err := newStatusesImage(statuses)
	if err != nil {
		return err
	}

	if err := c.OCIClient.Push(image, c.Repository.Tag(deployment).Name()); err != nil {
		return err
	}
	return nil
}

func extractStatuses(image v1.Image) ([]v1alpha1.DeploymentStatus, error) {
	var extracted bytes.Buffer
	err := crane.Export(image, &extracted)
	if err != nil {
		return nil, err
	}

	result := []v1alpha1.DeploymentStatus{}
	err = json.Unmarshal(extracted.Bytes(), &result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *StatusClient) GetStatuses(deployment string) ([]v1alpha1.DeploymentStatus, error) {
	image, err := c.OCIClient.Pull(c.Repository.Tag(deployment).Name())
	if err != nil {
		return nil, err
	}
	return extractStatuses(image)
}
