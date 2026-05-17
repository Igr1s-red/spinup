package engine

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"testing"
)

func TestValidName(t *testing.T) {
	valid := []string{"vm1", "my-vm", "vm_test", "a", "VM1", "a1b2c3", "abc-def_ghi"}
	for _, n := range valid {
		if !validName.MatchString(n) {
			t.Errorf("validName: expected %q to be valid", n)
		}
	}

	invalid := []string{"", "my vm", "vm/1", "vm.1", "../etc", "vm!", "vm@host", "vm:1"}
	for _, n := range invalid {
		if validName.MatchString(n) {
			t.Errorf("validName: expected %q to be invalid", n)
		}
	}
}

func TestSSHPort(t *testing.T) {
	tests := []struct {
		name   string
		config VirtualMachineConfig
		want   string
	}{
		{
			name:   "no networks",
			config: VirtualMachineConfig{},
			want:   "",
		},
		{
			name: "NAT with SSH port forward",
			config: VirtualMachineConfig{
				Networks: []NetworkInterfaceConfig{
					{NetworkMode: "nat", PortForwards: map[string]string{"22": "54321"}},
				},
			},
			want: "54321",
		},
		{
			name: "no port 22 forward",
			config: VirtualMachineConfig{
				Networks: []NetworkInterfaceConfig{
					{NetworkMode: "nat", PortForwards: map[string]string{"80": "8080"}},
				},
			},
			want: "",
		},
		{
			name: "nil PortForwards",
			config: VirtualMachineConfig{
				Networks: []NetworkInterfaceConfig{{NetworkMode: "nat"}},
			},
			want: "",
		},
		{
			name: "port read from first NIC only",
			config: VirtualMachineConfig{
				Networks: []NetworkInterfaceConfig{
					{NetworkMode: "nat", PortForwards: map[string]string{"22": "11111"}},
					{NetworkMode: "nat", PortForwards: map[string]string{"22": "22222"}},
				},
			},
			want: "11111",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.config.sshPort()
			if got != tt.want {
				t.Errorf("sshPort() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIsPortInUse(t *testing.T) {
	tests := []struct {
		errMsg string
		want   bool
	}{
		{"", false},
		{"bind: address already in use", true},
		{"listen tcp 127.0.0.1:8080: bind: address already in use", true},
		{"bind: can't assign requested address", true},
		{"permission denied", false},
		{"connection refused", false},
		{"no such file or directory", false},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%q", tt.errMsg), func(t *testing.T) {
			var err error
			if tt.errMsg != "" {
				err = fmt.Errorf("%s", tt.errMsg)
			}
			got := isPortInUse(err)
			if got != tt.want {
				t.Errorf("isPortInUse(%q) = %v, want %v", tt.errMsg, got, tt.want)
			}
		})
	}
}

func TestWriteConfigFile(t *testing.T) {
	vm := &VirtualMachine{
		Name:   "testvm",
		engine: &Engine{writer: io.Discard},
		path:   t.TempDir(),
		Config: VirtualMachineConfig{
			CPU:      2,
			Memory:   1024,
			DiskSize: 10,
			SSHUser:  "debian",
			Networks: []NetworkInterfaceConfig{
				{
					NetworkMode:  "nat",
					MACAddress:   "52:54:00:aa:bb:cc",
					PortForwards: map[string]string{"22": "12345"},
				},
			},
		},
	}

	if err := vm.writeConfigFile(); err != nil {
		t.Fatalf("writeConfigFile() error: %v", err)
	}

	// File must exist and contain valid JSON that round-trips correctly.
	b, err := os.ReadFile(vm.configPath())
	if err != nil {
		t.Fatalf("reading config: %v", err)
	}
	var loaded VirtualMachineConfig
	if err := json.Unmarshal(b, &loaded); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}
	if loaded.CPU != vm.Config.CPU {
		t.Errorf("CPU: got %d, want %d", loaded.CPU, vm.Config.CPU)
	}
	if loaded.Memory != vm.Config.Memory {
		t.Errorf("Memory: got %d, want %d", loaded.Memory, vm.Config.Memory)
	}
	if loaded.Networks[0].PortForwards["22"] != "12345" {
		t.Errorf("SSH port: got %q, want %q", loaded.Networks[0].PortForwards["22"], "12345")
	}

	// Temp file must be cleaned up.
	if _, err := os.Stat(vm.configPath() + ".tmp"); !os.IsNotExist(err) {
		t.Error("temp file was not removed after writeConfigFile")
	}

	// Mutate and rewrite — second pass must overwrite correctly.
	vm.Config.CPU = 4
	if err := vm.writeConfigFile(); err != nil {
		t.Fatalf("second writeConfigFile() error: %v", err)
	}
	b2, _ := os.ReadFile(vm.configPath())
	var loaded2 VirtualMachineConfig
	if err := json.Unmarshal(b2, &loaded2); err != nil {
		t.Fatalf("unmarshal after update: %v", err)
	}
	if loaded2.CPU != 4 {
		t.Errorf("after update CPU = %d, want 4", loaded2.CPU)
	}
}
