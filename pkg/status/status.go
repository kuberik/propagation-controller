package status

import (
	"bytes"
	"encoding/json"
	"time"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/static"
	"github.com/google/go-containerregistry/pkg/v1/types"
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

type Status struct {
	Version string    `json:"version,omitempty"`
	Healthy bool      `json:"healthy,omitempty"`
	Start   time.Time `json:"start,omitempty"`
}

func newStatusesImage(statuses []Status) (v1.Image, error) {
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

func (c *StatusClient) PublishStatuses(deployment string, statuses []Status) error {
	image, err := newStatusesImage(statuses)
	if err != nil {
		return err
	}

	if err := c.OCIClient.Push(image, c.Repository.Tag(deployment).Name()); err != nil {
		return err
	}
	return nil
}

func extractStatuses(image v1.Image) ([]Status, error) {
	var extracted bytes.Buffer
	err := crane.Export(image, &extracted)
	if err != nil {
		return nil, err
	}

	result := []Status{}
	err = json.Unmarshal(extracted.Bytes(), &result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *StatusClient) GetStatuses(deployment string) ([]Status, error) {
	image, err := c.OCIClient.Pull(c.Repository.Tag(deployment).Name())
	if err != nil {
		return nil, err
	}
	return extractStatuses(image)
}
