package cmd

import "errors"

var (
	ErrUnsupportedArchitecture    = errors.New("spinup: unsupported architecture")
	ErrUnsupportedOperatingSystem = errors.New("spinup: unsupported operating system")
)
