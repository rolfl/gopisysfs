package gopisysfs

import (
	"fmt"
	"path/filepath"
)

func (pi PiDetails) IsPort(port int) bool {
	for _, p := range pi.GPIOPorts {
		if p == port {
			return true
		}
	}
	return false
}

func (pi PiDetails) notAPort(port int) error {
	return fmt.Errorf("Port %v is not valid on this device: %s", port, pi.Model)
}

func (pi PiDetails) portFolder(port int) string {
	return filepath.Join("/sys/class/gpio", fmt.Sprintf("gpio%d", port))
}

func (pi PiDetails) GPIOResetAsync(int port) (<-chan error, error) {
	if !pi.IsPin(port) {
		return nil, pi.notAPort(port)
	}
	ret := make(chan error, 5)

	gpiodir := portFolder(port)
	if !checkFile(gpiodir) {
		ret <- nil
		return ret, nil
	}


	return 
}

