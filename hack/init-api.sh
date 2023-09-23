#!/bin/sh

kubebuilder create api --group "" --kind Propagation --version v1alpha1 --resource --controller
make manifests
