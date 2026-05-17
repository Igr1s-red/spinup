package engine

import (
	"path/filepath"
)

func (e *Engine) imagesPath() string {
	return filepath.Join(e.path, "image")
}

func (e *Engine) imagePath(name string) string {
	return filepath.Join(e.imagesPath(), name)
}

func (e *Engine) virtualMachinesPath() string {
	return filepath.Join(e.path, "virtual-machine")
}

func (e *Engine) virtualMachinePath(name string) string {
	return filepath.Join(e.virtualMachinesPath(), name)
}
