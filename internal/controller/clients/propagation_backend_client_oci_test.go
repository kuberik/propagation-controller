package clients

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/kuberik/propagation-controller/api/v1alpha1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func localRegistry(intercepts ...http.Handler) *httptest.Server {
	registry := registry.New()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, i := range intercepts {
			i.ServeHTTP(w, r)
		}
		registry.ServeHTTP(w, r)
	}))
}

func withRequestLog(requestLog *[]*http.Request) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*requestLog = append(*requestLog, r.Clone(context.TODO()))
	})
}

func TestPublishStatusOCI(t *testing.T) {
	input := v1alpha1.DeploymentStatus{
		Version: "a",
		Start:   metav1.NewTime(time.Date(2023, 9, 23, 8, 42, 0, 0, time.Local)),
	}

	registryServer := localRegistry()
	defer registryServer.Close()
	repository, err := name.NewRepository(fmt.Sprintf("%s/deployments/foo", strings.TrimPrefix(registryServer.URL, "http://")))
	assert.NoError(t, err)

	ociClient := NewOCIPropagationBackendClient(repository)
	client := NewPropagationClient(&ociClient)

	_, err = client.GetStatus("staging-frankfurt-1")
	assert.ErrorContains(t, err, "NAME_UNKNOWN")

	err = client.PublishStatus("staging-frankfurt-1", input)
	assert.NoError(t, err)
}

func TestPublishStatusOCICached(t *testing.T) {
	input := v1alpha1.DeploymentStatus{
		Version: "a",
		Start:   metav1.NewTime(time.Date(2023, 9, 23, 8, 42, 0, 0, time.Local)),
	}

	requestLog := []*http.Request{}
	registryServer := localRegistry(withRequestLog(&requestLog))
	defer registryServer.Close()
	repository, err := name.NewRepository(fmt.Sprintf("%s/deployments/foo", strings.TrimPrefix(registryServer.URL, "http://")))
	assert.NoError(t, err)

	propagationClientset := NewPropagationClientset(fake.NewFakeClient())
	propagation := v1alpha1.Propagation{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo",
			Namespace: "bar",
		},
		Spec: v1alpha1.PropagationSpec{
			Backend: v1alpha1.PropagationBackend{
				BaseUrl: fmt.Sprintf("oci://%s", repository),
			},
		},
	}
	client, err := propagationClientset.Propagation(propagation)
	assert.NoError(t, err)

	err = client.PublishStatus("staging-frankfurt-1", input)
	assert.NoError(t, err)

	requestCount := len(requestLog)

	// reinitialie client to check if caching is in effect
	client, err = propagationClientset.Propagation(propagation)
	assert.NoError(t, err)

	// cached call
	err = client.PublishStatus("staging-frankfurt-1", input)
	assert.NoError(t, err)
	assert.Equal(t, requestCount, len(requestLog))
}

func TestGetStatusOCI(t *testing.T) {
	want := v1alpha1.DeploymentStatus{
		Version: "a",
		Start:   metav1.NewTime(time.Date(2023, 9, 24, 8, 42, 0, 0, time.Local)),
	}

	registryServer := localRegistry()
	defer registryServer.Close()
	repository, err := name.NewRepository(fmt.Sprintf("%s/deployments/foo", strings.TrimPrefix(registryServer.URL, "http://")))
	assert.NoError(t, err)

	ociClient := NewOCIPropagationBackendClient(repository)
	client := NewPropagationClient(&ociClient)

	err = client.PublishStatus("prod", want)
	assert.NoError(t, err)

	status, err := client.GetStatus("prod")
	assert.NoError(t, err)
	assert.Equal(t, want, *status)
}

func TestGetStatusOCICached(t *testing.T) {
	initialStatus := v1alpha1.DeploymentStatus{
		Version: "a",
		Start:   metav1.NewTime(time.Date(2023, 9, 24, 8, 42, 0, 0, time.Local)),
	}

	requestLog := []*http.Request{}
	registryServer := localRegistry(withRequestLog(&requestLog))
	defer registryServer.Close()
	repository, err := name.NewRepository(fmt.Sprintf("%s/deployments/foo", strings.TrimPrefix(registryServer.URL, "http://")))
	assert.NoError(t, err)

	ociClientPublisher := NewOCIPropagationBackendClient(repository)
	clientPublisher := NewPropagationClient(&ociClientPublisher)

	err = clientPublisher.PublishStatus("prod", initialStatus)
	assert.NoError(t, err)

	propagationClientset := NewPropagationClientset(fake.NewFakeClient())
	propagation := v1alpha1.Propagation{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo",
			Namespace: "bar",
		},
		Spec: v1alpha1.PropagationSpec{
			Backend: v1alpha1.PropagationBackend{
				BaseUrl: fmt.Sprintf("oci://%s", repository),
			},
		},
	}
	clientGetter, err := propagationClientset.Propagation(propagation)
	assert.NoError(t, err)

	status, err := clientGetter.GetStatus("prod")
	assert.NoError(t, err)
	assert.Equal(t, initialStatus, *status)

	requestCount := len(requestLog)

	// reinitialie client to check if caching is in effect
	clientGetter, err = propagationClientset.Propagation(propagation)
	assert.NoError(t, err)

	// cached call
	status, err = clientGetter.GetStatus("prod")
	assert.NoError(t, err)
	assert.Equal(t, initialStatus, *status)
	assert.Equal(t, "/v2/", requestLog[requestCount].URL.Path)
	assert.Equal(t, "/v2/deployments/foo/statuses/prod/manifests/latest", requestLog[requestCount+1].URL.Path)
	assert.Equal(t, requestCount+2, len(requestLog))

	nextStatus := v1alpha1.DeploymentStatus{
		Version: "b",
		Start:   metav1.NewTime(time.Date(2023, 9, 25, 8, 42, 0, 0, time.Local)),
	}
	err = clientPublisher.PublishStatus("prod", nextStatus)
	assert.NoError(t, err)

	status, err = clientGetter.GetStatus("prod")
	assert.NoError(t, err)
	assert.Equal(t, nextStatus, *status)
}

func TestPropagate(t *testing.T) {
	registryServer := localRegistry()
	defer registryServer.Close()
	repository, err := name.NewRepository(fmt.Sprintf("%s/deployments/foo", strings.TrimPrefix(registryServer.URL, "http://")))
	assert.NoError(t, err)

	ociClient := NewOCIPropagationBackendClient(repository)
	client := NewPropagationClient(&ociClient)

	err = ociClient.Publish(ArtifactMetadata{
		Deployment: "staging",
		Type:       ManifestArtifactType,
		Version:    "abf1a799152d2655bbd7b4bf0b70422d7eda233f",
	}, &ociArtifact{image: empty.Image})
	assert.NoError(t, err)

	deploymentImage, err := ociClient.Fetch(
		ArtifactMetadata{
			Deployment: "staging",
			Type:       DeployArtifactType,
		})
	assert.ErrorContains(t, err, "NAME_UNKNOWN")
	assert.Nil(t, deploymentImage)

	err = client.Propagate("staging", "abf1a799152d2655bbd7b4bf0b70422d7eda233f")
	assert.NoError(t, err)

	deploymentImage, err = ociClient.Fetch(
		ArtifactMetadata{
			Deployment: "staging",
			Type:       DeployArtifactType,
		})
	assert.NoError(t, err)
	emptyImageDigest, err := empty.Image.Digest()
	assert.NoError(t, err)
	gotImageDigest, err := deploymentImage.DigestString()
	assert.NoError(t, err)
	assert.Equal(t, emptyImageDigest.String(), gotImageDigest)
}

func TestPropagateCached(t *testing.T) {
	requestLog := []*http.Request{}
	registryServer := localRegistry(withRequestLog(&requestLog))
	defer registryServer.Close()
	repository, err := name.NewRepository(fmt.Sprintf("%s/deployments/foo", strings.TrimPrefix(registryServer.URL, "http://")))
	assert.NoError(t, err)

	ociClient := NewOCIPropagationBackendClient(repository)

	err = ociClient.Publish(ArtifactMetadata{
		Deployment: "staging",
		Type:       ManifestArtifactType,
		Version:    "abf1a799152d2655bbd7b4bf0b70422d7eda233f",
	}, &ociArtifact{image: empty.Image})
	assert.NoError(t, err)

	propagationClientset := NewPropagationClientset(fake.NewFakeClient())
	propagation := v1alpha1.Propagation{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo",
			Namespace: "bar",
		},
		Spec: v1alpha1.PropagationSpec{
			Backend: v1alpha1.PropagationBackend{
				BaseUrl: fmt.Sprintf("oci://%s", repository),
			},
		},
	}
	client, err := propagationClientset.Propagation(propagation)
	assert.NoError(t, err)

	deploymentImage, err := ociClient.Fetch(
		ArtifactMetadata{
			Deployment: "staging",
			Type:       DeployArtifactType,
		})
	assert.ErrorContains(t, err, "NAME_UNKNOWN")
	assert.Nil(t, deploymentImage)

	err = client.Propagate("staging", "abf1a799152d2655bbd7b4bf0b70422d7eda233f")
	assert.NoError(t, err)

	requestCount := len(requestLog)

	// reinitialie client to check if caching is in effect
	client, err = propagationClientset.Propagation(propagation)
	assert.NoError(t, err)

	// cached call
	err = client.Propagate("staging", "abf1a799152d2655bbd7b4bf0b70422d7eda233f")
	assert.NoError(t, err)
	assert.Equal(t, requestCount, len(requestLog))
}
