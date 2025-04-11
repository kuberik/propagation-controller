package promotion

// import (
// 	"context"
// 	"fmt"
// 	"net/http"
// 	"strings"
// 	"testing"
// 	"time"

// 	"github.com/google/go-containerregistry/pkg/name"
// 	"github.com/google/go-containerregistry/pkg/v1/empty"
// 	"github.com/kuberik/propagation-controller/api/promotion/v1alpha1"
// 	"github.com/kuberik/propagation-controller/pkg/repo/config"
// 	testhelpers "github.com/kuberik/propagation-controller/pkg/test_helpers"
// 	"github.com/stretchr/testify/assert"
// 	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
// 	"sigs.k8s.io/controller-runtime/pkg/client/fake"
// )

// func withRequestLog(requestLog *[]*http.Request) http.Handler {
// 	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
// 		*requestLog = append(*requestLog, r.Clone(context.TODO()))
// 	})
// }

// func TestPromote(t *testing.T) {
// 	registryServer := testhelpers.LocalRegistry()
// 	defer registryServer.Close()
// 	repository, err := name.NewRepository(fmt.Sprintf("%s/deployments/foo", strings.TrimPrefix(registryServer.URL, "http://")))
// 	assert.NoError(t, err)

// 	ociClient := NewOCIPromotionBackendClient(repository)
// 	client := NewPromotionClient(&ociClient)

// 	err = ociClient.Publish(ArtifactMetadata{
// 		Deployment: "staging",
// 		Type:       ManifestArtifactType,
// 		Version:    "abf1a799152d2655bbd7b4bf0b70422d7eda233f",
// 	}, &ociArtifact{image: empty.Image})
// 	assert.NoError(t, err)

// 	deploymentImage, err := ociClient.Fetch(
// 		ArtifactMetadata{
// 			Deployment: "staging",
// 			Type:       DeployArtifactType,
// 		})
// 	assert.ErrorContains(t, err, "NAME_UNKNOWN")
// 	assert.Nil(t, deploymentImage)

// 	err = client.Promote("staging", "abf1a799152d2655bbd7b4bf0b70422d7eda233f")
// 	assert.NoError(t, err)

// 	deploymentImage, err = ociClient.Fetch(
// 		ArtifactMetadata{
// 			Deployment: "staging",
// 			Type:       DeployArtifactType,
// 		})
// 	assert.NoError(t, err)
// 	emptyImageDigest, err := empty.Image.Digest()
// 	assert.NoError(t, err)
// 	gotImageDigest, err := deploymentImage.DigestString()
// 	assert.NoError(t, err)
// 	assert.Equal(t, emptyImageDigest.String(), gotImageDigest)
// }

// func TestPromoteCached(t *testing.T) {
// 	requestLog := []*http.Request{}
// 	registryServer := testhelpers.LocalRegistry(withRequestLog(&requestLog))
// 	defer registryServer.Close()
// 	repository, err := name.NewRepository(fmt.Sprintf("%s/deployments/foo", strings.TrimPrefix(registryServer.URL, "http://")))
// 	assert.NoError(t, err)

// 	ociClient := NewOCIPromotionBackendClient(repository)

// 	err = ociClient.Publish(ArtifactMetadata{
// 		Deployment: "staging",
// 		Type:       ManifestArtifactType,
// 		Version:    "abf1a799152d2655bbd7b4bf0b70422d7eda233f",
// 	}, &ociArtifact{image: empty.Image})
// 	assert.NoError(t, err)

// 	promotionClientset := NewPromotionClientset(fake.NewFakeClient())
// 	promotion := v1alpha1.Promotion{
// 		ObjectMeta: metav1.ObjectMeta{
// 			Name:      "foo",
// 			Namespace: "bar",
// 		},
// 		Spec: v1alpha1.PromotionSpec{
// 			Backend: v1alpha1.PromotionBackend{
// 				BaseUrl: fmt.Sprintf("oci://%s", repository),
// 			},
// 		},
// 	}
// 	client, err := promotionClientset.Promotion(promotion)
// 	assert.NoError(t, err)

// 	deploymentImage, err := ociClient.Fetch(
// 		ArtifactMetadata{
// 			Deployment: "staging",
// 			Type:       DeployArtifactType,
// 		})
// 	assert.ErrorContains(t, err, "NAME_UNKNOWN")
// 	assert.Nil(t, deploymentImage)

// 	err = client.Promote("staging", "abf1a799152d2655bbd7b4bf0b70422d7eda233f")
// 	assert.NoError(t, err)

// 	requestCount := len(requestLog)

// 	// reinitialie client to check if caching is in effect
// 	client, err = promotionClientset.Promotion(promotion)
// 	assert.NoError(t, err)

// 	// cached call
// 	err = client.Promote("staging", "abf1a799152d2655bbd7b4bf0b70422d7eda233f")
// 	assert.NoError(t, err)
// 	assert.Equal(t, requestCount, len(requestLog))
// }

// func TestPublishConfigOCI(t *testing.T) {
// 	input := config.Config{
// 		Environments: []config.Environment{{
// 			Name: "dev",
// 			Waves: []config.Wave{{
// 				BakeTime: metav1.Duration{Duration: time.Hour},
// 			}},
// 			ReleaseCadence: config.ReleaseCadence{
// 				WaitTime: metav1.Duration{Duration: 2 * time.Hour},
// 			},
// 		}, {
// 			Name: "prod",
// 			Waves: []config.Wave{{
// 				BakeTime: metav1.Duration{Duration: time.Hour},
// 			}},
// 			ReleaseCadence: config.ReleaseCadence{
// 				// At 08:00 on Monday.
// 				Schedule: "0 8 * * 1",
// 			},
// 		}},
// 	}

// 	registryServer := testhelpers.LocalRegistry()
// 	defer registryServer.Close()
// 	repository, err := name.NewRepository(fmt.Sprintf("%s/deployments/foo", strings.TrimPrefix(registryServer.URL, "http://")))
// 	assert.NoError(t, err)

// 	ociClient := NewOCIPromotionBackendClient(repository)
// 	client := NewPromotionClient(&ociClient)

// 	_, err = client.GetConfig()
// 	assert.ErrorContains(t, err, "NAME_UNKNOWN")

// 	err = client.PublishConfig(input)
// 	assert.NoError(t, err)

// 	config, err := client.GetConfig()
// 	assert.NoError(t, err)
// 	assert.Equal(t, input, *config)
// }
