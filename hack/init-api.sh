#!/bin/sh

kubebuilder create api --group "" --kind Propagation --version v1alpha1 --resource --controller
kubebuilder create api --group "" --kind Health --version v1alpha1 --resource --controller=false
make generate manifests
