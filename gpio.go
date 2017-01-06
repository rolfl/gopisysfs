package gopisysfs

import (
	"fmt"
	"path/filepath"
	"time"
)

type GPIOFlag struct {
	flag bool
	err  error
}

type GPIOMode int

const (
	GPIOOutputLow GPIOMode = iota
	GPIOOutputHigh
	GPIOInput
)

type GPIOPort interface {
	IsGPIO() bool
	Reset() error
	Configure(GPIOMode) error
	IsInput() (bool, error)
	GetValue() (bool, error)
	SetValue(bool) error
	Values() (<-chan bool, error)
}

type gport struct {
	host     *pi
	port     int
	sport    string
	folder   string
	export   string
	unexport string
}

func newGPIO(host *pi, port int) *gport {
	sport := fmt.Sprintf("%d", port)
	gpio := host.gpiodir
	folder := filepath.Join(gpio, fmt.Sprintf("gpio%s", sport))
	export := filepath.Join(gpio, "export")
	unexport := filepath.Join(gpio, "unexport")
	return &gport{
		host:     host,
		port:     port,
		sport:    sport,
		folder:   folder,
		export:   export,
		unexport: unexport,
	}
}

func (p *gport) String() string {
	return p.folder
}

func (p *gport) IsGPIO() bool {
	return p.isGPIO() == nil
}

func (p *gport) Configure(GPIOMode) error {
	return nil
}
func (p *gport) IsInput() (bool, error) {
	return false, nil
}
func (p *gport) GetValue() (bool, error) {
	return false, nil
}
func (p *gport) SetValue(bool) error {
	return nil
}
func (p *gport) Values() (<-chan bool, error) {
	return nil, nil
}

func (p *gport) isGPIO() error {
	return nil
}

// GPIOResetAsync will reset the specified port, if it exists, and return a nil value on the returned channel when the reset is complete
func (p *gport) resetAsync(timeout time.Duration) (<-chan error, error) {
	if err := p.isGPIO(); err != nil {
		return nil, err
	}
	if err := writeFile(p.unexport, p.sport); err != nil {
		return nil, err
	}

	return awaitFileRemove(p.folder, timeout)
}

// GPIOResetAsync will reset the specified port and only return when it is complete
func (p *gport) Reset() error {
	ch, err := p.resetAsync(forever)
	if err != nil {
		return err
	}
	return <-ch
}
