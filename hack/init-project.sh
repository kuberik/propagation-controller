#!/bin/sh

mkdir -p propagation-controller
cd propagation-controller
kubebuilder init --domain kuberik.io --repo github.com/kuberik/propagation-controller
