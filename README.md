# spinup

Spin up Linux VMs with QEMU.

## Requirements

- Linux or macOS host.
- [QEMU](https://www.qemu.org) installed. spinup uses `qemu-img`, `qemu-system-x86_64` (AMD64), or `qemu-system-aarch64` (ARM64).

## Install

```shell
go install github.com/Igr1s-red/spinup@latest
```

## Quick start

```shell
# Pull an image, create and start a VM
spinup run myvm -i debian:bookworm

# SSH in
spinup ssh myvm

# Run a one-off command
spinup exec myvm -- uname -a

# Stop and remove
spinup stop myvm
spinup remove myvm
```

## Available images

| Image | Description |
|---|---|
| `arch:latest` | Arch Linux (latest Reproducible Builds cloud image) |
| `debian:bullseye` | Debian 11 (Bullseye) — oldstable |
| `debian:bookworm` | Debian 12 (Bookworm) — stable |
| `debian:trixie` | Debian 13 (Trixie) — testing |
| `fedora:latest` | Fedora (latest stable, resolved at pull time) |
| `ubuntu:focal` | Ubuntu 20.04 LTS |
| `ubuntu:jammy` | Ubuntu 22.04 LTS |
| `ubuntu:noble` | Ubuntu 24.04 LTS |

Images are downloaded on first use, or manually with `spinup pull <image>`.

## Commands

### VM lifecycle

```shell
spinup run <name> -i <image> [flags]   # create + start
spinup start <name> [--wait] [--all]   # start (--all starts every stopped VM)
spinup stop <name> [--all]             # ACPI powerdown, falls back to SIGKILL
spinup restart <name>
spinup remove <name>                   # rm is an alias
spinup rename <old> <new>
spinup pause <name>                    # freeze CPU in memory
spinup resume <name>
spinup prune [--force]                 # remove all stopped VMs (dry run without --force)
```

**`spinup run` flags:**

| Flag | Description |
|---|---|
| `-i`, `--image` | Image to use (required) |
| `-c`, `--cpu` | vCPU count (default 1) |
| `-m`, `--memory` | RAM in MiB (default 512) |
| `-d`, `--disk-size` | Disk in GB (default 10) |
| `--network` | NIC definition, repeatable. Format: `<mode>[:<hostport>-<vmport>,...]`. Modes: `nat`, `bridged`, `internal`, `host-only` |
| `--share` | Expose a host directory via VirtIO 9P. Format: `/host/path:tag` |
| `--profile` | Apply a saved profile as defaults (flags override) |
| `--user-data` | Path to a cloud-init user-data file |
| `--wait` | Block until SSH is ready |
| `--wait-port` | Also wait for this VM port (e.g. `80`) |
| `--no-start` | Create the VM but don't start it |
| `--ephemeral` | Automatically remove the VM when stopped |

### SSH and interaction

```shell
spinup ssh <name>                      # interactive shell
spinup ssh <name> --command            # print raw ssh command for scripting: $(spinup ssh vm1 --command)
spinup exec <name> -- <cmd>            # run a command and stream output
spinup cp <vm>:/remote/path ./local    # download a file
spinup cp ./local <vm>:/remote/path   # upload a file
spinup console <name>                  # serial console (requires socat; Ctrl+] to exit)
spinup copy-id <name> [--key file]     # install your public key into the VM
spinup ip <name> [--all]              # print VM IP address
spinup mount add <vm> <remote> <local> # mount via sshfs
spinup mount remove <local>
```

### Observability

```shell
spinup list [--json]                   # ls is an alias
spinup inspect <name>                  # full JSON config + runtime info
spinup stats <name>                    # CPU/memory/disk stats (live QMP when running)
spinup logs <name> [-f]               # stream QEMU log; Ctrl+C exits follow mode
spinup port <name> [vm-port]          # print host port mapped to a VM port
spinup wait <name> --running|--stopped|--wait-port <port>
```

### Images

```shell
spinup images                          # list available images and pull status
spinup pull <image>                    # download an image
spinup update <image>                  # re-pull to get the latest build
spinup image-rm <image>               # delete a pulled image to reclaim disk space
```

### Snapshots

VM must be stopped. Snapshots are stored inside the qcow2 disk.

```shell
spinup snapshot create <vm> [name]    # auto-names with timestamp if omitted
spinup snapshot list <vm> [--json]
spinup snapshot restore <vm> <name>
spinup snapshot delete <vm> <name>
```

### Port forwards

Changes take effect on the next start.

```shell
spinup port-forward add <vm> <host>-<vm-port>   # e.g. 8080-80
spinup port-forward remove <vm> <vm-port>
spinup port-forward list <vm>
```

### Advanced

```shell
spinup clone <src> <dst>              # linked copy-on-write clone (src must be stopped)
spinup resize <name> <gb>             # grow disk (VM must be stopped)
spinup balloon <name> <mib>           # adjust live memory balloon target
spinup export <name> [file]           # export disk as standalone compressed qcow2
spinup import <file> <name>           # create a VM from an external qcow2
spinup set-key <name>                 # rotate SSH key pair (takes effect on next boot)
spinup ssh-config write <name>        # add VM to ~/.ssh/config (enables: ssh <name>)
spinup ssh-config remove <name>
spinup autostart enable|disable|status <name>
spinup profile create|list|show|delete
spinup doctor                         # check environment (QEMU, KVM/HVF, sshfs, etc.)
```

## Contributing

### Requirements

- [Go](https://golang.org) installed.
- [Task](https://taskfile.dev) installed (optional; `go build .` works without it).

```shell
go build .       # build
go test ./...    # test
go vet ./...     # vet
```
