package engine

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"fmt"
	"strings"

	"github.com/Igr1s-red/spinup/cloudinit"
	goxssh "golang.org/x/crypto/ssh"
)

// generateSSHKey creates an Ed25519 key pair, writes both files to the VM
// data directory, and returns the public key in authorized_keys format.
func (v *VirtualMachine) generateSSHKey() ([]byte, error) {
	pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate Ed25519 key: %w", err)
	}
	privPEMBlock, err := goxssh.MarshalPrivateKey(privKey, "spinup")
	if err != nil {
		return nil, fmt.Errorf("marshal private key: %w", err)
	}
	sshPub, err := goxssh.NewPublicKey(pubKey)
	if err != nil {
		return nil, fmt.Errorf("marshal public key: %w", err)
	}
	pubKeyBytes := goxssh.MarshalAuthorizedKey(sshPub)

	if err := v.writeFile(v.privateKeyPath(), pem.EncodeToMemory(privPEMBlock), 0600); err != nil {
		return nil, err
	}
	if err := v.writeFile(v.publicKeyPath(), pubKeyBytes, 0644); err != nil {
		return nil, err
	}
	return pubKeyBytes, nil
}

// buildCloudInit writes the cloud-init NoCloud ISO for the VM.
// instanceID is embedded in meta-data; pass v.Name for a stable id or
// v.Name+"-"+randomHex(8) to force cloud-init to re-run on next boot.
func (v *VirtualMachine) buildCloudInit(userData, instanceID string) error {
	return cloudinit.New(cloudinit.NewOptions{
		MetaData:      fmt.Sprintf("instance-id: %s\nlocal-hostname: %s\n", instanceID, v.Name),
		Name:          v.cloudInitPath(),
		NetworkConfig: cloudInitNetworkConfig,
		UserData:      userData,
	})
}

// defaultCloudInitUserData returns the standard user-data for pubKeyBytes.
func defaultCloudInitUserData(pubKeyBytes []byte) string {
	return fmt.Sprintf(cloudInitTemplate, strings.TrimSpace(string(pubKeyBytes)))
}
