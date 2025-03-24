package config

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Config struct {
	Environments []Environment `json:"environment,omitempty"`
}

func (c *Config) DeploymentBakeTime(deployment string) time.Duration {
	for _, env := range c.Environments {
		for _, wave := range env.Waves {
			for _, d := range wave.Deployments {
				if d == deployment {
					return wave.BakeTime.Duration
				}
			}
		}
	}
	return 0
}

type Environment struct {
	Name  string `json:"name,omitempty"`
	Waves []Wave `json:"waves,omitempty"`
}

type Wave struct {
	BakeTime metav1.Duration `json:"bakeTime,omitempty"`
	// TODO: this should be populated from manifests
	Deployments []string `json:"deployments,omitempty"`
}
