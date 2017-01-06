package gopisysfs

import (
	"fmt"
	"time"
)

func (pi Pi) IsPort(port int) bool {
	for _, p := range pi.GPIOPorts {
		if p == port {
			return true
		}
	}
	return false
}

func (pi Pi) notAPort(port int) error {
	return fmt.Errorf("Port %v is not valid on this device: %s", port, pi.Model)
}

func (pi Pi) portFolder(port int) string {
	return file("sys", "class", "gpio", fmt.Sprintf("gpio%d", port))
}

// GPIOResetAsync will reset the specified port, if it exists, and return a nil value on the returned channel when the reset is complete
func (pi Pi) GPIOResetAsync(port int, timeout time.Duration) (<-chan error, error) {
	if !pi.IsPort(port) {
		return nil, pi.notAPort(port)
	}
	return awaitFileRemove(pi.portFolder(port), timeout)
}

// GPIOResetAsync will reset the specified portand only return when it is complete
func (pi Pi) GPIOReset(port int) error {
	ch, err := pi.GPIOResetAsync(port, forever)
	if err != nil {
		return err
	}
	return <-ch
}
