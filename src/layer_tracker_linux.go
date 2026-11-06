//go:build linux
// +build linux

package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/gdamore/tcell"
	"golang.org/x/sys/unix"
)

func layerTrackingSupported() bool { return true }

func findVialHID() (string, error) {
	if path := os.Getenv("NM_HIDRAW"); path != "" {
		return path, nil
	}

	paths, _ := filepath.Glob("/dev/hidraw*")
	sort.Strings(paths)
	for _, path := range paths {
		name := filepath.Base(path)
		desc, err := ioutil.ReadFile(filepath.Join("/sys/class/hidraw", name, "device", "report_descriptor"))
		if err != nil {
			continue
		}
		// Usage page 0xFF60, usage 0x61 is the Vial raw HID interface.
		if bytes.Contains(desc, []byte{0x06, 0x60, 0xff, 0x09, 0x61}) {
			return path, nil
		}
	}
	return "", fmt.Errorf("no Vial raw HID interface found (checked /dev/hidraw*; set NM_HIDRAW to override)")
}

func startLayerTracker(scr tcell.Screen, kb *vilKeyboard) (*layerTracker, error) {
	path, err := findVialHID()
	if err != nil {
		return nil, err
	}

	fd, err := unix.Open(path, unix.O_RDWR|unix.O_NONBLOCK, 0)
	if err != nil {
		return nil, fmt.Errorf("%s: %v", path, err)
	}
	t := &layerTracker{
		fd:          fd,
		scr:         scr,
		kb:          kb,
		layer:       -1,
		stop:        make(chan struct{}),
		done:        make(chan struct{}),
		prevPressed: map[[2]int]bool{},
	}

	// Vial hides the switch matrix unless the keyboard is unlocked.
	resp, err := t.query(0xfe, 0x05)
	if err != nil {
		unix.Close(fd)
		return nil, fmt.Errorf("%s: %v", path, err)
	}
	if resp[0] != 1 {
		unix.Close(fd)
		return nil, fmt.Errorf("Vial is locked; unlock the keyboard in Vial first")
	}

	go t.run()
	return t, nil
}

func waitHID(fd int, events int16, timeoutMS int) error {
	fds := []unix.PollFd{{Fd: int32(fd), Events: events}}
	n, err := unix.Poll(fds, timeoutMS)
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("HID timeout")
	}
	return nil
}

func (t *layerTracker) query(command, value byte) ([32]byte, error) {
	var out [32]byte
	var msg [32]byte
	msg[0] = command
	msg[1] = value

	var buf [64]byte
	for {
		n, err := unix.Read(t.fd, buf[:])
		if n <= 0 || err != nil {
			break
		}
	}

	for tries := 0; tries < 3; tries++ {
		n, err := unix.Write(t.fd, msg[:])
		if err == nil && n == len(msg) {
			break
		}
		if err != nil && err != unix.EAGAIN {
			return out, err
		}
		if tries == 2 {
			return out, fmt.Errorf("HID write failed")
		}
		if err := waitHID(t.fd, unix.POLLOUT, 100); err != nil {
			return out, err
		}
	}

	if err := waitHID(t.fd, unix.POLLIN, 100); err != nil {
		return out, err
	}
	n, err := unix.Read(t.fd, buf[:])
	if err != nil {
		return out, err
	}
	if n <= 0 {
		return out, fmt.Errorf("empty HID response")
	}
	copy(out[:], buf[:n])
	return out, nil
}

func (t *layerTracker) readMatrix() (map[[2]int]bool, error) {
	resp, err := t.query(0x02, 0x03)
	if err != nil {
		return nil, err
	}
	if resp[0] != 0x02 || resp[1] != 0x03 {
		return nil, fmt.Errorf("unexpected VIA matrix response")
	}
	return t.pressedFromState(resp[2:]), nil
}

func (t *layerTracker) run() {
	defer close(t.done)
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-t.stop:
			return
		case <-ticker.C:
		}
		pressed, err := t.readMatrix()
		if err != nil {
			continue
		}
		t.updateActive(pressed)
	}
}

func (t *layerTracker) Close() {
	t.closeOnce.Do(func() {
		close(t.stop)
		<-t.done
		if t.fd >= 0 {
			unix.Close(t.fd)
			t.fd = -1
		}
	})
}
