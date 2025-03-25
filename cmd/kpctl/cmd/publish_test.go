package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/kuberik/propagation-controller/pkg/clients"
	"github.com/kuberik/propagation-controller/pkg/repo/config"
	testhelpers "github.com/kuberik/propagation-controller/pkg/test_helpers"
	"github.com/stretchr/testify/assert"
)

func TestPublishConfig(t *testing.T) {
	registryServer := testhelpers.LocalRegistry()
	defer registryServer.Close()

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	backend := fmt.Sprintf("oci://%s/myapp", strings.TrimPrefix(registryServer.URL, "http://"))
	configContent := fmt.Sprintf(`
backend: %s
environments:
  - name: development
    waves:
      - bakeTime: 30m
        deployments:
          - development-eu-1
  - name: staging
    waves:
      - bakeTime: 1h
        deployments:
          - staging-eu-1
  - name: production
    waves:
      - bakeTime: 2h
        deployments:
          - production-eu-1
      - bakeTime: 1h
        deployments:
          - production-eu-2
          - production-us-1
`, backend)
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	assert.NoError(t, err)

	// TODO: test authentication
	// tmpDir := t.TempDir()
	// dockerConfig := filepath.Join(tmpDir, ".docker.config.json")
	// os.Setenv("DOCKER_CONFIG", dockerConfig)
	// _, err := os.Create(dockerConfig)
	// assert.NoError(t, err)

	err = publishConfig(configPath)
	assert.NoError(t, err)

	backendClient, err := clients.NewPropagationBackendClient(backend)
	assert.NoError(t, err)
	client := clients.NewPropagationClient(backendClient)
	gotConfig, err := client.GetConfig()
	assert.NoError(t, err)

	wantJson, err := yamlToJSON([]byte(configContent))
	assert.NoError(t, err)
	wantConfig := &config.Config{}
	err = json.Unmarshal(wantJson, wantConfig)
	assert.NoError(t, err)

	assert.Equal(t, wantConfig, gotConfig)
}

func TestFindAllConfigs(t *testing.T) {
	dummyGitRepo := t.TempDir()
	// Create a dummy git repo
	_, err := git.PlainInit(dummyGitRepo, false)
	if err != nil {
		t.Fatalf("failed to initialize git repository: %v", err)
	}
	assert.NoError(t, os.MkdirAll(filepath.Join(dummyGitRepo, ".kuberik"), 0755))
	configFile1 := filepath.Join(dummyGitRepo, ".kuberik", "config1.yaml")
	_, err = os.Create(configFile1)
	assert.NoError(t, err)

	assert.NoError(t, os.MkdirAll(filepath.Join(dummyGitRepo, ".kuberik", "subpath"), 0755))
	configFile2 := filepath.Join(dummyGitRepo, ".kuberik", "subpath", "config2.yaml")
	_, err = os.Create(configFile2)
	assert.NoError(t, err)

	assert.NoError(t, os.MkdirAll(filepath.Join(dummyGitRepo, ".kuberik", "another", "subpath"), 0755))
	configFile3 := filepath.Join(dummyGitRepo, ".kuberik", "another", "subpath", "config2.yaml")
	_, err = os.Create(configFile3)
	assert.NoError(t, err)

	gitSubdir := filepath.Join(dummyGitRepo, "subdir")
	if err := os.Mkdir(gitSubdir, 0755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}

	got, err := findConfigs(dummyGitRepo)
	assert.NoError(t, err)
	assert.ElementsMatch(t, []string{configFile1, configFile2, configFile3}, got)
}
