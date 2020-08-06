package cluster

import (
	"github.com/rancher/rancher/pkg/agent/steve"
)

var running bool

func RunControllers() error {
	if running {
		return nil
	}

	if err := steve.Run(); err != nil {
		return err
	}

	running = true
	return nil
}
