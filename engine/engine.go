package engine

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/sha512"
	"errors"
	"fmt"
	"hash"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"

	"github.com/Igr1s-red/spinup/qemu"
	"github.com/cheggaaa/pb/v3"
)

// NewOptions configures the Engine.
type NewOptions struct {
	QEMUExecutableName string
	Path               string
	// Writer receives progress output. Defaults to io.Discard if nil.
	Writer io.Writer
}

// CreateVirtualMachineOptions configures a new VM.
type CreateVirtualMachineOptions struct {
	CPU      int
	Image    string
	Memory   int
	Name     string
	DiskSize int
	// Networks specifies NICs in order; index 0 is primary (SSH).
	// Defaults to a single NAT NIC when empty.
	Networks []NetworkInterfaceConfig
	// SharedFolders are host directories to expose via VirtIO 9P.
	SharedFolders []SharedFolder
	// UserData overrides the default cloud-init user-data when non-empty.
	UserData string
	// Ephemeral marks the VM for automatic removal when stopped.
	Ephemeral bool
}

// Engine is the top-level runtime for managing VMs and images.
type Engine struct {
	qemu            *qemu.QEMU
	images          map[string]*Image
	path            string
	virtualMachines map[string]*VirtualMachine
	writer          io.Writer
}

// printf writes formatted progress output to the engine's writer.
// Unexported: callers within this package use it; cmd uses os.Stderr directly.
func (e *Engine) printf(format string, a ...interface{}) {
	fmt.Fprintf(e.writer, format, a...)
}

// randomMAC generates a random locally administered unicast MAC address.
func RandomMAC() (string, error) {
	buf := make([]byte, 6)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate MAC address: %w", err)
	}
	// Set locally administered bit, clear multicast bit.
	buf[0] = (buf[0] | 0x02) & 0xfe
	return fmt.Sprintf("%02x:%02x:%02x:%02x:%02x:%02x",
		buf[0], buf[1], buf[2], buf[3], buf[4], buf[5]), nil
}

// ── Download ──────────────────────────────────────────────────────────────────

// httpClient is used for all image downloads. No timeout is set because large
// image downloads may take many minutes on slow connections.
var httpClient = &http.Client{}

// download streams url to dest, showing a progress bar.
// It uses a single GET request; if Content-Length is absent the bar shows
// indeterminate progress rather than failing.
func (e *Engine) download(url, dest string) error {
	e.printf("Downloading %s\n", url)

	resp, err := httpClient.Get(url)
	if err != nil {
		return fmt.Errorf("download %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: HTTP %d %s", url, resp.StatusCode, resp.Status)
	}

	f, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("create %s: %w", dest, err)
	}

	bar := pb.Full.New(int(resp.ContentLength)) // -1 if unknown; pb handles it
	bar.SetWriter(e.writer)
	bar.Start()

	_, copyErr := io.Copy(f, bar.NewProxyReader(resp.Body))
	bar.Finish()

	if copyErr != nil {
		f.Close()
		os.Remove(dest)
		return fmt.Errorf("write %s: %w", dest, copyErr)
	}

	if err := f.Close(); err != nil {
		os.Remove(dest)
		return fmt.Errorf("close %s: %w", dest, err)
	}

	return nil
}

// ── Checksum verification ─────────────────────────────────────────────────────

// verifyChecksum validates dest against a hex-encoded digest.
// The algorithm is inferred from the digest length:
//
//	64 hex chars  → SHA-256
//	128 hex chars → SHA-512
//
// An empty checksum is accepted without verification (some resolvers return ""
// when the checksum endpoint is temporarily unavailable).
func (e *Engine) verifyChecksum(checksum, dest string) error {
	if checksum == "" {
		return nil
	}

	var h hash.Hash
	var algoName string

	switch len(checksum) {
	case 64:
		h = sha256.New()
		algoName = "SHA-256"
	case 128:
		h = sha512.New()
		algoName = "SHA-512"
	default:
		return fmt.Errorf("unrecognised checksum length %d "+
			"(want 64 hex chars for SHA-256 or 128 for SHA-512)", len(checksum))
	}

	e.printf("Verifying %s checksum...\n", algoName)

	f, err := os.Open(dest)
	if err != nil {
		return fmt.Errorf("open for checksum: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(h, f); err != nil {
		return fmt.Errorf("hash %s: %w", dest, err)
	}

	got := fmt.Sprintf("%x", h.Sum(nil))
	if !strings.EqualFold(got, checksum) {
		// Remove the file — it's corrupt or wrong.
		os.Remove(dest)
		return fmt.Errorf("%w\n  got:  %s\n  want: %s", ErrInvalidChecksum, got, checksum)
	}

	return nil
}

// ── Image registry ────────────────────────────────────────────────────────────

func (e *Engine) buildImageRegistry() {
	type entry struct {
		key, name, version, desc, user, dir string
		resolver                            func(string) (string, string, error)
	}

	entries := []entry{
		{
			key: "arch:latest", name: "arch", version: "latest",
			desc: "Arch Linux (latest, official Reproducible Builds cloud image)",
			user: "arch", dir: "arch-latest",
			resolver: resolveArch(),
		},
		{
			key: "debian:bullseye", name: "debian", version: "bullseye",
			desc: "Debian 11 (Bullseye) — oldstable",
			user: "debian", dir: "debian-bullseye",
			resolver: resolveDebian("bullseye"),
		},
		{
			key: "debian:bookworm", name: "debian", version: "bookworm",
			desc: "Debian 12 (Bookworm) — stable",
			user: "debian", dir: "debian-bookworm",
			resolver: resolveDebian("bookworm"),
		},
		{
			key: "debian:trixie", name: "debian", version: "trixie",
			desc: "Debian 13 (Trixie) — testing",
			user: "debian", dir: "debian-trixie",
			resolver: resolveDebian("trixie"),
		},
		{
			key: "fedora:latest", name: "fedora", version: "latest",
			desc: "Fedora (latest stable release, resolved at pull time)",
			user: "fedora", dir: "fedora-latest",
			resolver: resolveFedora(),
		},
		{
			key: "ubuntu:focal", name: "ubuntu", version: "focal",
			desc: "Ubuntu 20.04 LTS (Focal Fossa)",
			user: "ubuntu", dir: "ubuntu-focal",
			resolver: resolveUbuntu("focal"),
		},
		{
			key: "ubuntu:jammy", name: "ubuntu", version: "jammy",
			desc: "Ubuntu 22.04 LTS (Jammy Jellyfish)",
			user: "ubuntu", dir: "ubuntu-jammy",
			resolver: resolveUbuntu("jammy"),
		},
		{
			key: "ubuntu:noble", name: "ubuntu", version: "noble",
			desc: "Ubuntu 24.04 LTS (Noble Numbat)",
			user: "ubuntu", dir: "ubuntu-noble",
			resolver: resolveUbuntu("noble"),
		},
	}

	e.images = make(map[string]*Image, len(entries))
	for _, en := range entries {
		e.images[en.key] = &Image{
			Name:        en.name,
			Version:     en.version,
			Description: en.desc,
			Dynamic:     true,
			Resolver:    en.resolver,
			engine:      e,
			path:        e.imagePath(en.dir),
			sshUser:     en.user,
		}
	}
}

// FindImage returns the named image or nil.
func (e *Engine) FindImage(name string) *Image {
	return e.images[name]
}

// ListImages returns all images sorted by name then version.
func (e *Engine) ListImages() []*Image {
	imgs := make([]*Image, 0, len(e.images))
	for _, img := range e.images {
		imgs = append(imgs, img)
	}
	sort.Slice(imgs, func(i, j int) bool {
		if imgs[i].Name != imgs[j].Name {
			return imgs[i].Name < imgs[j].Name
		}
		return imgs[i].Version < imgs[j].Version
	})
	return imgs
}

// ── VM registry ───────────────────────────────────────────────────────────────

// FindVirtualMachine returns the named VM or nil.
func (e *Engine) FindVirtualMachine(name string) *VirtualMachine {
	return e.virtualMachines[name]
}

// ListVirtualMachines returns all loaded VMs. Order is not guaranteed.
func (e *Engine) ListVirtualMachines() []*VirtualMachine {
	vms := make([]*VirtualMachine, 0, len(e.virtualMachines))
	for _, vm := range e.virtualMachines {
		vms = append(vms, vm)
	}
	return vms
}

func (e *Engine) loadVirtualMachines() error {
	vmsPath := e.virtualMachinesPath()

	entries, err := os.ReadDir(vmsPath)
	if errors.Is(err, os.ErrNotExist) {
		e.virtualMachines = map[string]*VirtualMachine{}
		return nil
	}
	if err != nil {
		return fmt.Errorf("list VMs directory %s: %w", vmsPath, err)
	}

	e.virtualMachines = make(map[string]*VirtualMachine, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		name := entry.Name()
		vm := &VirtualMachine{
			Name:   name,
			engine: e,
			path:   e.virtualMachinePath(name),
		}

		if err := vm.loadConfigFile(); err != nil {
			return fmt.Errorf("load config for VM %q: %w", name, err)
		}

		e.virtualMachines[name] = vm
	}

	return nil
}

// ── Constructor ───────────────────────────────────────────────────────────────

// New creates a new Engine. It validates QEMU availability and loads all
// existing VMs from disk.
func New(opts *NewOptions) (*Engine, error) {
	writer := opts.Writer
	if writer == nil {
		writer = io.Discard
	}

	e := &Engine{
		path:   opts.Path,
		writer: writer,
	}

	var err error
	e.qemu, err = qemu.New(qemu.NewOptions{
		ExecutableName: opts.QEMUExecutableName,
	})
	if err != nil {
		return nil, err
	}

	e.buildImageRegistry()

	if err := e.loadVirtualMachines(); err != nil {
		return nil, err
	}

	return e, nil
}
