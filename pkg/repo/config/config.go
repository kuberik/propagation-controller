package config

import (
	"time"

	"slices"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type PropagationConfig struct {
	Backend string `json:"backend,omitempty"`
	Config  `json:",inline"`
}

type Config struct {
	Environments []Environment `json:"environments,omitempty"`
}

func (c *Config) DeploymentBakeTime(deployment string) time.Duration {
	for _, env := range c.Environments {
		for _, wave := range env.Waves {
			if slices.Contains(wave.Deployments, deployment) {
				return wave.BakeTime.Duration
			}
		}
	}
	return 0
}

type Environment struct {
	Name           string         `json:"name,omitempty"`
	Waves          []Wave         `json:"waves,omitempty"`
	ReleaseCadence ReleaseCadence `json:"releaseCadence,omitempty"`
}

type Wave struct {
	BakeTime metav1.Duration `json:"bakeTime,omitempty"`
	// TODO: this should be populated from manifests
	Deployments []string `json:"deployments,omitempty"`
}

type ReleaseCadence struct {
	Schedule string          `json:"schedule,omitempty"`
	WaitTime metav1.Duration `json:"waitTime,omitempty"`
}
