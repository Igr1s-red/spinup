package engine

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// Profile stores a named set of VM creation defaults.
// Any field left at its zero value means "use the command-line default".
type Profile struct {
	Name          string                   `json:"name"`
	Image         string                   `json:"image,omitempty"`
	CPU           int                      `json:"cpu,omitempty"`
	Memory        int                      `json:"memory_mib,omitempty"`
	DiskSize      int                      `json:"disk_size_gb,omitempty"`
	Networks      []NetworkInterfaceConfig `json:"networks,omitempty"`
	SharedFolders []SharedFolder           `json:"shared_folders,omitempty"`
	UserData      string                   `json:"user_data,omitempty"`
}

func (e *Engine) profileDir() string {
	return filepath.Join(e.path, "profiles")
}

func (e *Engine) profilePath(name string) string {
	return filepath.Join(e.profileDir(), name+".json")
}

// SaveProfile writes a profile to disk.
func (e *Engine) SaveProfile(p *Profile) error {
	if p.Name == "" || !validName.MatchString(p.Name) {
		return fmt.Errorf("profile %w", ErrInvalidName)
	}

	if err := os.MkdirAll(e.profileDir(), 0755); err != nil {
		return fmt.Errorf("create profiles directory: %w", err)
	}

	b, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}

	tmp := e.profilePath(p.Name) + ".tmp"
	if err := os.WriteFile(tmp, b, 0644); err != nil {
		return err
	}
	if err := os.Rename(tmp, e.profilePath(p.Name)); err != nil {
		return err
	}

	e.printf("Profile %q saved\n", p.Name)
	return nil
}

// LoadProfile reads a profile from disk.
func (e *Engine) LoadProfile(name string) (*Profile, error) {
	b, err := os.ReadFile(e.profilePath(name))
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("%w: %q", ErrProfileNotFound, name)
	}
	if err != nil {
		return nil, fmt.Errorf("read profile %q: %w", name, err)
	}

	var p Profile
	if err := json.Unmarshal(b, &p); err != nil {
		return nil, fmt.Errorf("parse profile %q: %w", name, err)
	}
	return &p, nil
}

// DeleteProfile removes a profile from disk.
func (e *Engine) DeleteProfile(name string) error {
	err := os.Remove(e.profilePath(name))
	if errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("%w: %q", ErrProfileNotFound, name)
	}
	if err != nil {
		return fmt.Errorf("delete profile %q: %w", name, err)
	}
	e.printf("Profile %q deleted\n", name)
	return nil
}

// ListProfiles returns all saved profiles sorted by name.
func (e *Engine) ListProfiles() ([]*Profile, error) {
	entries, err := os.ReadDir(e.profileDir())
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("list profiles: %w", err)
	}

	var profiles []*Profile
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		name := entry.Name()[:len(entry.Name())-5] // strip .json
		p, err := e.LoadProfile(name)
		if err != nil {
			return nil, err
		}
		profiles = append(profiles, p)
	}

	sort.Slice(profiles, func(i, j int) bool {
		return profiles[i].Name < profiles[j].Name
	})
	return profiles, nil
}

// ApplyProfile merges profile defaults into opts.
// Explicit values in opts (non-zero) take precedence over profile values.
func ApplyProfile(opts *CreateVirtualMachineOptions, p *Profile) {
	if p == nil {
		return
	}
	if opts.Image == "" && p.Image != "" {
		opts.Image = p.Image
	}
	if opts.CPU == 0 && p.CPU != 0 {
		opts.CPU = p.CPU
	}
	if opts.Memory == 0 && p.Memory != 0 {
		opts.Memory = p.Memory
	}
	if opts.DiskSize == 0 && p.DiskSize != 0 {
		opts.DiskSize = p.DiskSize
	}
	if len(opts.Networks) == 0 && len(p.Networks) != 0 {
		opts.Networks = p.Networks
	}
	if len(opts.SharedFolders) == 0 && len(p.SharedFolders) != 0 {
		opts.SharedFolders = p.SharedFolders
	}
	if opts.UserData == "" && p.UserData != "" {
		opts.UserData = p.UserData
	}
}
