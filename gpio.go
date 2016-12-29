package gopisysfs

import (
	"fmt"
	"time"
)

const forever = 100 * 365 * 24 * time.Hour

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
	ret := make(chan error, 5)

	gpiodir := pi.portFolder(port)
	if checkFile(gpiodir) {
		tout := time.After(timeout)
		go func() {
			ticker := time.NewTicker(50 * time.Millisecond).C
			for {
				if !checkFile(gpiodir) {
					// successful reset
					ret <- nil
					return
				}
				select {
				case <-tout:
					ret <- fmt.Errorf("Timeout resetting port %v after %v", port, timeout)
					return
				case <-ticker:
				}
			}
		}()
	} else {
		// folder is already missing, no need for polling.
		ret <- nil
	}

	return ret, nil
}

// GPIOResetAsync will reset the specified portand only return when it is complete
func (pi Pi) GPIOReset(port int) error {
	ch, err := pi.GPIOResetAsync(port, forever)
	if err != nil {
		return err
	}
	return <-ch
}
