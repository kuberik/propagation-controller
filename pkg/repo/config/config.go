package config

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Config struct {
	Environments []Environment `json:"environment,omitempty"`
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
