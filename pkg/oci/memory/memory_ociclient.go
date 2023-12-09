package memory

import (
	"fmt"
	"sync"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/kuberik/propagation-controller/pkg/oci"
)

var _ oci.OCIClient = &MemoryOCIClient{}

type MemoryOCIClient struct {
	images map[string]v1.Image
	mutex  sync.Mutex
}

func NewMemoryOCIClient() MemoryOCIClient {
	return MemoryOCIClient{
		images: make(map[string]v1.Image),
	}
}

// Pull implements oci.OCIClient.
func (c *MemoryOCIClient) Pull(tag string) (v1.Image, error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	image, ok := c.images[tag]
	if !ok {
		return nil, fmt.Errorf("image not found")
	}
	return image, nil
}

// Digest implements oci.OCIClient.
func (c *MemoryOCIClient) Digest(tag string) (string, error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	image, ok := c.images[tag]
	if !ok {
		return "", fmt.Errorf("image not found")
	}

	digest, err := image.Digest()
	if err != nil {
		return "", err
	}
	return digest.String(), nil
}

// Push implements oci.OCIClient.
func (c *MemoryOCIClient) Push(image v1.Image, tag string) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.images[tag] = image
	return nil
}
