package oci

import (
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
)

//go:generate sh -c "$(go env GOPATH)/bin/mockgen -destination fake/mock_ociclient.go -package fake github.com/kuberik/propagation-controller/pkg/oci OCIClient"
type OCIClient interface {
	Pull(string) (v1.Image, error)
	Push(v1.Image, string) error
}

var _ OCIClient = &CraneClient{}

type CraneClient struct {
	options []crane.Option
}

// Pull implements OCIClient.
func (c *CraneClient) Pull(tag string) (v1.Image, error) {
	return crane.Pull(tag, c.options...)
}

// Push implements OCIClient.
func (c *CraneClient) Push(img v1.Image, tag string) error {
	return crane.Push(img, tag, c.options...)
}

type RemoteImage struct {
	tag    name.Tag
	client OCIClient
}

func NewRemoteImage(image string, client OCIClient) (*RemoteImage, error) {
	tag, err := name.NewTag(image)
	if err != nil {
		return nil, err
	}
	return &RemoteImage{
		tag:    tag,
		client: client,
	}, nil
}

func (i *RemoteImage) Pull() (v1.Image, error) {
	return i.client.Pull(i.tag.Name())
}

func (i *RemoteImage) Push(img v1.Image) error {
	return i.client.Push(img, i.tag.Name())
}
