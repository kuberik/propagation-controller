package status

import (
	"bytes"
	"encoding/json"
	reflect "reflect"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/crane"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/stretchr/testify/assert"
	gomock "go.uber.org/mock/gomock"
)

func TestPublishStatuses(t *testing.T) {
	input := []Status{{
		Version: "a",
		Start:   time.Date(2023, 9, 23, 8, 42, 0, 0, time.UTC),
	}, {
		Version: "b",
		Start:   time.Date(2023, 9, 23, 9, 42, 0, 0, time.UTC),
	}}

	ctrl := gomock.NewController(t)
	ociClient := NewMockOCIClient(ctrl)
	ociClient.EXPECT().Push(gomock.Cond(func(x any) bool {
		image, ok := x.(v1.Image)
		if !ok {
			return false
		}

		var extracted bytes.Buffer
		err := crane.Export(image, &extracted)
		assert.NoError(t, err)

		got := []Status{}
		err = json.Unmarshal(extracted.Bytes(), &got)
		assert.NoError(t, err)

		return reflect.DeepEqual(input, got)
	}), "registry.example.local/deployments/foo:staging")

	client, err := NewStatusClient("registry.example.local/deployments/foo", ociClient)
	assert.NoError(t, err)

	err = client.PublishStatuses("staging", input)
	assert.NoError(t, err)
}
