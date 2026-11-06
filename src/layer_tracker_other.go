//go:build !linux
// +build !linux

package main

import (
	"fmt"

	"github.com/gdamore/tcell"
)

func layerTrackingSupported() bool { return false }

func startLayerTracker(scr tcell.Screen, kb *vilKeyboard) (*layerTracker, error) {
	return nil, fmt.Errorf("physical layer tracking is only supported on Linux")
}

func (t *layerTracker) Close() {}
