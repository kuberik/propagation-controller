package status

import (
	reflect "reflect"
	"testing"
	"time"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/kuberik/propagation-controller/api/v1alpha1"
	fakeoci "github.com/kuberik/propagation-controller/pkg/oci/fake"
	"github.com/stretchr/testify/assert"
	gomock "go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestPublishStatuses(t *testing.T) {
	input := []v1alpha1.DeploymentStatusReport{{
		Version: "a",
		Start:   metav1.NewTime(time.Date(2023, 9, 23, 8, 42, 0, 0, time.Local)),
	}, {
		Version: "b",
		Start:   metav1.NewTime(time.Date(2023, 9, 23, 9, 42, 0, 0, time.Local)),
	}}

	ctrl := gomock.NewController(t)
	ociClient := fakeoci.NewMockOCIClient(ctrl)
	ociClient.EXPECT().Push(gomock.Cond(func(x any) bool {
		image, ok := x.(v1.Image)
		if !ok {
			return false
		}

		got, err := extractStatuses(image)
		assert.NoError(t, err)

		return reflect.DeepEqual(input, got)
	}), "registry.example.local/deployments/foo:staging")

	client, err := NewStatusClient("registry.example.local/deployments/foo", ociClient)
	assert.NoError(t, err)

	err = client.PublishStatuses("staging", input)
	assert.NoError(t, err)
}

func TestGetStatuses(t *testing.T) {
	want := []v1alpha1.DeploymentStatusReport{{
		Version: "a",
		Start:   metav1.NewTime(time.Date(2023, 9, 24, 8, 42, 0, 0, time.Local)),
	}, {
		Version: "b",
		Start:   metav1.NewTime(time.Date(2023, 9, 24, 9, 42, 0, 0, time.Local)),
	}}

	ctrl := gomock.NewController(t)
	ociClient := fakeoci.NewMockOCIClient(ctrl)
	statusesImage, err := newStatusesImage(want)
	assert.NoError(t, err)
	ociClient.EXPECT().Pull("registry.example.local/deployments/foo:prod").Return(statusesImage, nil)

	client, err := NewStatusClient("registry.example.local/deployments/foo", ociClient)
	assert.NoError(t, err)

	statuses, err := client.GetStatuses("prod")
	assert.NoError(t, err)
	assert.Equal(t, want, statuses)
}
