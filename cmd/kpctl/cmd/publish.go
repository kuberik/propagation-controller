package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/go-git/go-git/v5"
	"github.com/kuberik/propagation-controller/pkg/clients"
	"github.com/kuberik/propagation-controller/pkg/repo/config"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// publishCmd represents the publish command
var publishCmd = &cobra.Command{
	Use:   "publish",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("publish called")
	},
}

func init() {
	rootCmd.AddCommand(publishCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// publishCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// publishCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
	// publishConfig()
}

func yamlToJSON(yamlContent []byte) ([]byte, error) {
	config := map[string]any{}
	err := yaml.Unmarshal([]byte(yamlContent), &config)
	if err != nil {
		return nil, fmt.Errorf("failed to parse yaml: %w", err)
	}

	jsonContent, err := json.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal json: %w", err)
	}
	return jsonContent, nil
}

func publishConfig(path string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	contentJson, err := yamlToJSON(content)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	cfg := config.PropagationConfig{}
	if err := json.Unmarshal(contentJson, &cfg); err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	backendClient, err := clients.NewPropagationBackendClient(cfg.Backend)
	if err != nil {
		return fmt.Errorf("failed to create backend client: %w", err)
	}
	client := clients.NewPropagationClient(backendClient)
	if err := client.PublishConfig(cfg.Config); err != nil {
		return fmt.Errorf("failed to publish config: %w", err)
	}

	return nil
}

func findConfigs(repoPath string) ([]string, error) {
	repo, err := git.PlainOpenWithOptions(repoPath, &git.PlainOpenOptions{DetectDotGit: true})
	if err != nil {
		return nil, fmt.Errorf("failed to open git repository: %w", err)
	}
	worktree, err := repo.Worktree()
	if err != nil {
		return nil, fmt.Errorf("failed to get git worktree: %w", err)
	}
	rootDir := worktree.Filesystem.Root()

	// find all files in .kuberik directory
	configs := []string{}
	err = filepath.Walk(filepath.Join(rootDir, ".kuberik"), func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && slices.Contains([]string{".yaml", ".yml"}, filepath.Ext(path)) {
			configs = append(configs, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to find configs: %w", err)
	}

	return configs, nil
}

func PublishConfigs() error {
	configs, err := findConfigs(".")
	if err != nil {
		return fmt.Errorf("failed to find configs: %w", err)
	}

	for _, config := range configs {
		err := publishConfig(config)
		if err != nil {
			return fmt.Errorf("failed to publish config: %w", err)
		}
	}

	return nil
}
