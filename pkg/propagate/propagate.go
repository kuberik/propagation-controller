package propagate

import (
	"github.com/kuberik/propagation-controller/pkg/oci"
)

func Propagate(source oci.RemoteImage, destination oci.RemoteImage) error {
	image, err := source.Pull()
	if err != nil {
		return err
	}

	if err := destination.Push(image); err != nil {
		return err
	}
	return nil
}
