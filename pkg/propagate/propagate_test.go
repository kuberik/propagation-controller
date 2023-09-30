package propagate

import (
	"testing"

	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/kuberik/propagation-controller/pkg/oci"
	fakeoci "github.com/kuberik/propagation-controller/pkg/oci/fake"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestPropagate(t *testing.T) {
	ctrl := gomock.NewController(t)
	sourceOCIClient := fakeoci.NewMockOCIClient(ctrl)
	sourceOCIClient.EXPECT().Pull("registry.example.local/manifests/staging:abf1a799152d2655bbd7b4bf0b70422d7eda233f").Return(empty.Image, nil)
	destinationOCIClient := fakeoci.NewMockOCIClient(ctrl)
	destinationOCIClient.EXPECT().Push(empty.Image, "registry.example.local/deployments/staging:latest")

	source, err := oci.NewRemoteImage("registry.example.local/manifests/staging:abf1a799152d2655bbd7b4bf0b70422d7eda233f", sourceOCIClient)
	assert.NoError(t, err)
	destination, err := oci.NewRemoteImage("registry.example.local/deployments/staging:latest", destinationOCIClient)
	assert.NoError(t, err)

	err = Propagate(*source, *destination)
	assert.NoError(t, err)
}
