package gopisysfs

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type GPIOFlag struct {
	flag bool
	err  error
}

type GPIOMode int

const (
	GPIOInput GPIOMode = iota
	GPIOOutput
	GPIOOutputLow
	GPIOOutputHigh

	// from https://www.kernel.org/doc/Documentation/gpio/sysfs.txt
	direction_in     = "in"
	direction_out    = "out"
	direction_outlow = "low"
	direction_outhi  = "high"

	// the longest time to wait for an operation to complete
	timelimit = time.Second

	low  = "0"
	high = "1"
)

type GPIOPort interface {
	State() string
	IsEnabled() bool
	Enable() error
	Reset() error
	SetMode(GPIOMode) error
	IsOutput() (bool, error)
	SetValue(bool) error
	Value() (bool, error)
	Values() (<-chan bool, error)
}

type gport struct {
	mu        sync.Mutex
	host      *pi
	port      int
	sport     string
	folder    string
	value     string
	direction string
	edge      string
	export    string
	unexport  string
}

func newGPIO(host *pi, port int) *gport {

	sport := fmt.Sprintf("%d", port)
	gpio := host.gpiodir
	folder := filepath.Join(gpio, fmt.Sprintf("gpio%s", sport))
	export := filepath.Join(gpio, "export")
	unexport := filepath.Join(gpio, "unexport")

	return &gport{
		mu:        sync.Mutex{},
		host:      host,
		port:      port,
		sport:     sport,
		folder:    folder,
		value:     filepath.Join(folder, "value"),
		direction: filepath.Join(folder, "direction"),
		edge:      filepath.Join(folder, "edge"),
		export:    export,
		unexport:  unexport,
	}
}

func (p *gport) String() string {
	return p.folder
}

func (p *gport) IsEnabled() bool {

	defer p.unlock(p.lock())

	return checkFile(p.folder)
}

func (p *gport) Enable() error {

	defer p.unlock(p.lock())

	if checkFile(p.folder) {
		return nil
	}

	info("GPIO Enabling %v\n", p)

	if err := writeFile(p.export, p.sport); err != nil {
		return err
	}

	start := time.Now()
	// wait for folder to arrive....
	ch, err := awaitFileCreate(p.folder, timelimit)
	if err != nil {
		return err
	}
	if err := <-ch; err != nil {
		return err
	}
	// delay a bit.
	<-time.After(pollInterval * 2)
	// and for all control files to exist and be readable
	// there's an issue with timeouts perhaps.... but that's OK.
	for _, fname := range []string{p.folder, p.direction, p.value, p.edge} {
		for {
			remaining := timelimit - time.Since(start)
			info("GPIO Enabling %v checking file %v state (timeout limit %v)\n", p, fname, remaining)
			if checkFile(fname) {
				// check writable.... invalid data will be ignored, but permissions won't
				if err := writeFile(fname, " "); err == nil || !os.IsPermission(err) {
					info("GPIO Enabling %v checking file %v state\n", p, fname)
					break
				}
			}
			select {
			case <-time.After(remaining):
				return fmt.Errorf("Timed out enabling GPIO %v - %v not yet writable", p.sport, fname)
			case <-time.After(pollInterval):
				// next cycle
			}
		}

	}

	info("GPIO Enabled %v\n", p)

	return nil
}

func (p *gport) Reset() error {

	defer p.unlock(p.lock())

	if !checkFile(p.folder) {
		// already reset
		return nil
	}
	info("GPIO Resetting  %v\n", p)
	if err := writeFile(p.unexport, p.sport); err != nil {
		return err
	}
	ch, err := awaitFileRemove(p.folder, timelimit)
	if err != nil {
		return err
	}

	if err := <-ch; err != nil {
		return err
	}

	// wait for the file to be removed, and then return
	info("GPIO Reset  %v\n", p)
	return nil

}

// GPIOResetAsync will reset the specified port and only return when it is complete
// Configure will
func (p *gport) SetMode(mode GPIOMode) error {

	defer p.unlock(p.lock())

	err := p.checkEnabled()
	if err != nil {
		return err
	}

	switch mode {
	case GPIOInput:
		return p.writeDirection(direction_in)
	case GPIOOutput:
		return p.writeDirection(direction_out)
	case GPIOOutputHigh:
		return p.writeDirection(direction_outhi)
	case GPIOOutputLow:
		return p.writeDirection(direction_outlow)
	}
	return fmt.Errorf("GPIOMode %v does not exist")
}

func (p *gport) IsOutput() (bool, error) {

	defer p.unlock(p.lock())

	err := p.checkEnabled()
	if err != nil {
		return false, err
	}
	d, err := p.readDirection()
	if err != nil {
		return false, err
	}
	return d != "in", nil
}

func (p *gport) State() string {

	defer p.unlock(p.lock())

	base := fmt.Sprintf("GPIO %v: ", p.sport)
	if !checkFile(p.folder) {
		return base + "Reset"
	}

	dir, err := p.readDirection()
	if err != nil {
		return fmt.Sprintf("%v%v", base, err)
	}
	val, err := p.readValue()
	if err != nil {
		return fmt.Sprintf("%v%v", base, err)
	}

	return fmt.Sprintf("%v %v with value %v", base, dir, val)
}

func (p *gport) Value() (bool, error) {

	defer p.unlock(p.lock())

	err := p.checkEnabled()
	if err != nil {
		return false, err
	}

	d, err := p.readValue()
	if err != nil {
		return false, err
	}

	return d == "1", nil
}

func (p *gport) SetValue(value bool) error {

	defer p.unlock(p.lock())

	err := p.checkEnabled()
	if err != nil {
		return err
	}

	info("GPIO Set Value on %v to %v\n", p, value)

	val := low
	if value {
		val = high
	}

	return p.writeValue(val)

}

func (p *gport) Values() (<-chan bool, error) {
	defer p.unlock(p.lock())
	return nil, nil
}

func (p *gport) writeDirection(direction string) error {
	info("GPIO Setting mode on  %v to %v\n", p, direction)

	return writeFile(p.direction, direction)
}

func (p *gport) readDirection() (string, error) {
	return readFile(p.direction)
}

func (p *gport) writeValue(value string) error {
	return writeFile(p.value, value)
}

func (p *gport) readValue() (string, error) {
	return readFile(p.value)
}

func (p *gport) checkEnabled() error {
	if checkFile(p.folder) {
		return nil
	}
	return fmt.Errorf("GPIO %v is not enabled", p.port)
}

func (p *gport) lock() bool {
	p.mu.Lock()
	return true
}

func (p *gport) unlock(bool) {
	p.mu.Unlock()
}
