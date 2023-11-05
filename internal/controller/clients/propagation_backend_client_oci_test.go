package clients

import (
	reflect "reflect"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/kuberik/propagation-controller/api/v1alpha1"
	fakeoci "github.com/kuberik/propagation-controller/pkg/oci/fake"
	"github.com/stretchr/testify/assert"
	gomock "go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestPublishStatusOCI(t *testing.T) {
	input := v1alpha1.DeploymentStatus{
		Version: "a",
		Start:   metav1.NewTime(time.Date(2023, 9, 23, 8, 42, 0, 0, time.Local)),
	}

	ctrl := gomock.NewController(t)
	ociClient := fakeoci.NewMockOCIClient(ctrl)
	ociClient.EXPECT().Push(gomock.Cond(func(x any) bool {
		image, ok := x.(v1.Image)
		if !ok {
			return false
		}

		got, err := extractStatus(image)
		assert.NoError(t, err)

		return reflect.DeepEqual(input, *got)
	}), "registry.example.local/deployments/foo/statuses/staging-frankfurt-1:latest")

	repository, err := name.NewRepository("registry.example.local/deployments/foo")
	assert.NoError(t, err)
	client := PropagationBackendOCIClient{
		Repository: repository,
		OCIClient:  ociClient,
	}

	err = client.PublishStatus("staging-frankfurt-1", input)
	assert.NoError(t, err)
}

func TestGetStatusOCI(t *testing.T) {
	want := v1alpha1.DeploymentStatus{
		Version: "a",
		Start:   metav1.NewTime(time.Date(2023, 9, 24, 8, 42, 0, 0, time.Local)),
	}

	ctrl := gomock.NewController(t)
	ociClient := fakeoci.NewMockOCIClient(ctrl)
	statusImage, err := newStatusImage(want)
	assert.NoError(t, err)
	ociClient.EXPECT().Pull("registry.example.local/deployments/foo/statuses/prod:latest").Return(statusImage, nil)

	repository, err := name.NewRepository("registry.example.local/deployments/foo")
	assert.NoError(t, err)
	client := PropagationBackendOCIClient{
		Repository: repository,
		OCIClient:  ociClient,
	}

	status, err := client.GetStatus("prod")
	assert.NoError(t, err)
	assert.Equal(t, want, *status)
}
