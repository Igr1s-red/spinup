package engine

import (
	"fmt"
)

// SetKey rotates the VM's Ed25519 SSH key pair and rebuilds the cloud-init ISO.
// The VM must be stopped. A fresh instance-id is embedded in the ISO so that
// cloud-init re-runs on next boot and installs the new authorized key.
func (v *VirtualMachine) SetKey() error {
	if err := v.requireStopped(); err != nil {
		return err
	}

	v.engine.printf("Rotating SSH key for VM %q\n", v.Name)

	pubKeyBytes, err := v.generateSSHKey()
	if err != nil {
		return err
	}

	// Use a fresh instance-id so cloud-init re-runs and applies the new key.
	instanceID := v.Name + "-" + randomHex(8)
	if err := v.buildCloudInit(defaultCloudInitUserData(pubKeyBytes), instanceID); err != nil {
		return fmt.Errorf("rebuild cloud-init ISO: %w", err)
	}

	v.engine.printf("SSH key rotated for %q — new key takes effect on next boot\n", v.Name)
	return nil
}
