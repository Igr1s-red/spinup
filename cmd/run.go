package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Igr1s-red/spinup/engine"
	"github.com/spf13/cobra"
)

func newRunCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run [name]",
		Short: "Create and start a new virtual machine",
		Args:  cobra.ExactArgs(1),
		Example: `  Create and start a Debian 12 VM:
    spinup run vm1 -i debian:bookworm

  Use a saved profile (overrides are allowed):
    spinup run vm1 --profile dev
    spinup run vm1 --profile dev -m 8192

  4 CPUs, 4 GiB RAM, 20 GB disk:
    spinup run vm1 -i debian:bookworm -c 4 -m 4096 -d 20

  NAT with port forward, then wait for SSH:
    spinup run vm1 -i debian:bookworm --network nat:8080-80 --wait

  Bridged (VM gets its own LAN IP):
    spinup run vm1 -i debian:bookworm --network bridged

  Two NICs — NAT primary + bridged secondary:
    spinup run vm1 -i debian:bookworm --network nat --network bridged

  Share a host directory (mount tag "src"):
    spinup run vm1 -i debian:bookworm --share /home/user/src:src

  Custom cloud-init user-data:
    spinup run vm1 -i debian:bookworm --user-data ./cloud-config.yaml`,
		RunE: func(cmd *cobra.Command, args []string) error {
			globalOptions, err := newGlobalOptions(cmd)
			if err != nil {
				return err
			}

			profileName, _ := cmd.Flags().GetString("profile")
			image, _ := cmd.Flags().GetString("image")
			cpu, _ := cmd.Flags().GetInt("cpu")
			memory, _ := cmd.Flags().GetInt("memory")
			diskSize, _ := cmd.Flags().GetInt("disk-size")
			networks, _ := cmd.Flags().GetStringArray("network")
			shares, _ := cmd.Flags().GetStringArray("share")
			userData, _ := cmd.Flags().GetString("user-data")
			wait, _ := cmd.Flags().GetBool("wait")
			waitTimeout, _ := cmd.Flags().GetDuration("wait-timeout")
			waitPort, _ := cmd.Flags().GetString("wait-port")
			noStart, _ := cmd.Flags().GetBool("no-start")
			ephemeral, _ := cmd.Flags().GetBool("ephemeral")

			if err := runRun(globalOptions, args[0], runFlags{
				profile:     profileName,
				image:       image,
				cpu:         cpu,
				memory:      memory,
				diskSize:    diskSize,
				networks:    networks,
				shares:      shares,
				userData:    userData,
				wait:        wait,
				waitTimeout: waitTimeout,
				waitPort:    waitPort,
				noStart:     noStart,
				ephemeral:   ephemeral,
			}); err != nil {
				fmt.Printf("Error: %s\n", err)
				os.Exit(1)
			}
			return nil
		},
	}

	cmd.Flags().String("profile", "", "apply a saved profile as defaults (flags override profile)")
	cmd.Flags().StringP("image", "i", "", "image to use")
	cmd.Flags().IntP("cpu", "c", 0, "number of vCPUs (default 1 if no profile)")
	cmd.Flags().IntP("memory", "m", 0, "RAM in MiB (default 512 if no profile)")
	cmd.Flags().IntP("disk-size", "d", 0, "disk size in GB (default 10 if no profile)")
	cmd.Flags().StringArray("network", nil,
		"NIC definition (repeatable). Format: <mode>[:<hostport>-<vmport>,...]\n"+
			"Modes: nat, bridged, internal, host-only")
	cmd.Flags().StringArray("share", nil,
		"share a host directory via VirtIO 9P. Format: /host/path:tag")
	cmd.Flags().String("user-data", "", "path to a cloud-init user-data file")
	cmd.Flags().Bool("wait", false, "block until SSH is accepting connections")
	cmd.Flags().Duration("wait-timeout", 2*time.Minute, "max time to wait for SSH or --wait-port")
	cmd.Flags().String("wait-port", "", "also wait for this VM port to accept connections (e.g. 80)")
	cmd.Flags().Bool("no-start", false, "create the VM but do not start it")
	cmd.Flags().Bool("ephemeral", false, "automatically remove the VM when it is stopped")

	return cmd
}

type runFlags struct {
	profile     string
	image       string
	cpu         int
	memory      int
	diskSize    int
	networks    []string
	shares      []string
	userData    string
	wait        bool
	waitTimeout time.Duration
	waitPort    string
	noStart     bool
	ephemeral   bool
}

func runRun(globalOpts *globalOptions, name string, flags runFlags) error {
	eng, err := newEngine(globalOpts)
	if err != nil {
		return err
	}

	opts := engine.CreateVirtualMachineOptions{
		Name:      name,
		Image:     flags.image,
		CPU:       flags.cpu,
		Memory:    flags.memory,
		DiskSize:  flags.diskSize,
		Ephemeral: flags.ephemeral,
	}

	// Apply profile first; flags override it below.
	if flags.profile != "" {
		p, err := eng.LoadProfile(flags.profile)
		if err != nil {
			return err
		}
		engine.ApplyProfile(&opts, p)
	}

	// Flags beat profile: re-apply non-zero flag values.
	if flags.cpu > 0 {
		opts.CPU = flags.cpu
	}
	if flags.memory > 0 {
		opts.Memory = flags.memory
	}
	if flags.diskSize > 0 {
		opts.DiskSize = flags.diskSize
	}
	if flags.image != "" {
		opts.Image = flags.image
	}

	// Apply defaults if still zero.
	if opts.CPU == 0 {
		opts.CPU = 1
	}
	if opts.Memory == 0 {
		opts.Memory = 512
	}
	if opts.DiskSize == 0 {
		opts.DiskSize = 10
	}

	// Network interfaces.
	if len(flags.networks) > 0 {
		for _, n := range flags.networks {
			opts.Networks = append(opts.Networks, parseNetworkFlag(n))
		}
	}

	// Shared folders.
	for _, s := range flags.shares {
		host, tag, ok := strings.Cut(s, ":")
		if !ok {
			fmt.Fprintf(os.Stderr, "warning: skipping malformed --share %q (need /host/path:tag)\n", s)
			continue
		}
		opts.SharedFolders = append(opts.SharedFolders, engine.SharedFolder{
			HostPath: host, Tag: tag,
		})
	}

	// User data.
	if flags.userData != "" {
		b, err := os.ReadFile(flags.userData)
		if err != nil {
			return fmt.Errorf("reading user-data: %w", err)
		}
		opts.UserData = string(b)
	}

	vm, err := eng.CreateVirtualMachine(opts)
	if err != nil {
		return err
	}

	if flags.noStart {
		fmt.Printf("VM %q created. Use: spinup start %s\n", vm.Name, vm.Name)
		return nil
	}

	if err := vm.Start(); err != nil {
		return err
	}

	fmt.Printf("\nVM %q is running\n", vm.Name)
	fmt.Printf("  SSH:    spinup ssh %s\n", vm.Name)
	fmt.Printf("  Logs:   spinup logs %s -f\n", vm.Name)
	fmt.Printf("  Stop:   spinup stop %s\n\n", vm.Name)

	// Auto-write SSH config entry so tools like scp and rsync work immediately.
	if err := vm.SSHConfigEntry(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: SSH config entry not written: %s\n", err)
	}

	if flags.wait {
		ctx, cancel := context.WithTimeout(context.Background(), flags.waitTimeout)
		defer cancel()
		if err := vm.WaitForSSH(ctx); err != nil {
			return err
		}
	}

	if flags.waitPort != "" {
		hostPort, err := resolveVMPort(vm, flags.waitPort)
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), flags.waitTimeout)
		defer cancel()
		return vm.WaitForPort(ctx, hostPort)
	}

	return nil
}

// parseNetworkFlag parses "--network nat:8080-80,9090-9090" into a config.
func parseNetworkFlag(s string) engine.NetworkInterfaceConfig {
	mode, forwards, _ := strings.Cut(s, ":")
	cfg := engine.NetworkInterfaceConfig{
		NetworkMode:  mode,
		PortForwards: map[string]string{},
	}
	for _, fwd := range strings.Split(forwards, ",") {
		fwd = strings.TrimSpace(fwd)
		if fwd == "" {
			continue
		}
		hostPort, vmPort, ok := strings.Cut(fwd, "-")
		if !ok {
			fmt.Fprintf(os.Stderr, "warning: ignoring malformed port forward %q\n", fwd)
			continue
		}
		cfg.PortForwards[vmPort] = hostPort
	}
	return cfg
}
