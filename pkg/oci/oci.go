package oci

import (
	"github.com/google/go-containerregistry/pkg/crane"
	v1 "github.com/google/go-containerregistry/pkg/v1"
)

//go:generate sh -c "$(go env GOPATH)/bin/mockgen -destination fake/mock_ociclient.go -package fake github.com/kuberik/propagation-controller/pkg/oci OCIClient"
type OCIClient interface {
	Pull(string) (v1.Image, error)
	Digest(string) (string, error)
	Push(v1.Image, string) error
}

var _ OCIClient = &CraneClient{}

type CraneClient struct {
	Options []crane.Option
}

// Pull implements OCIClient.
func (c *CraneClient) Pull(tag string) (v1.Image, error) {
	return crane.Pull(tag, c.Options...)
}

// Digest implements OCIClient.
func (c *CraneClient) Digest(tag string) (string, error) {
	return crane.Digest(tag, c.Options...)
}

// Push implements OCIClient.
func (c *CraneClient) Push(img v1.Image, tag string) error {
	return crane.Push(img, tag, c.Options...)
}
