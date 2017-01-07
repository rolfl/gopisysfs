package gopisysfs

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
)

const (
	sys_model    = "sys/firmware/devicetree/base/model"
	sys_gpio     = "sys/class/gpio"
	proc_cpuinfo = "proc/cpuinfo"
)

//Pi contains information describing the Pi model we are running on
type Pi interface {
	Model() string
	Revision() string
	P1GPIOPorts() []int
	GetPort(int) (GPIOPort, error)
}

// GetDetails returns the details of the Pi that is currently being run on
func GetPi() Pi {
	getdetails.Do(initOnce)
	return host
}

// GetDetailsFor returns the Pi internal details given a specific model and hardware revision
func GetDetailsFor(revision, model string) Pi {
	return buildPi(revision, model)
}

type pi struct {
	mu            sync.Mutex
	model         string
	revision      string
	controllerdir string
	gpiodir       string
	gpioports     []int
	portctrl      map[int]*gport
}

func init() {
	setOnPi()
	setModelMaps()
	setAvailableGPIOs()
}

var onpi bool

// setOnPi determines whether we are actually running on a real pi board, and not some other system
// setOnPi is called from the init() function
func setOnPi() {
	// don't use file(...) mechanism here. Need absolute file reference.
	compat, err := readFile("/sys/firmware/devicetree/base/compatible")
	if err != nil {
		return
	}
	// brcm matches the broadcom compat mechanism, which almost certainly means we are running on a pi.
	if regexp.MustCompile(`.*\bbrcm\b.*`).MatchString(compat) {
		onpi = true
	}
}

// IsOnPi returns true if this code is (probably) running on a Raspberry Pi.
func IsOnPi() bool {
	return onpi
}

// from http://www.raspberrypi-spy.co.uk/2012/06/simple-guide-to-the-rpi-gpio-header-and-pins/

// GPIO26HeaderV1 enumerates the pins available on the 26 pin P1 header on V1.0 raspberry pi systems
var GPIO26HeaderV1 = []int{
	14, 15, 18, 23, 24, 25, 8, 7,
	0, 1, 4, 17, 21, 22, 10, 9, 11,
}

// GPIO26HeaderV2 enumerates the pins available on the 26 pin P1 header on V2.0 raspberry pi systems
var GPIO26HeaderV2 = []int{
	14, 15, 18, 23, 24, 25, 8, 7,
	2, 3, 4, 17, 27, 22, 10, 9, 11,
}

// GPIO40HeaderV1 enumerates the pins available on the 40 pin P1 header on Model B+ and Pi2 and Pi3 models
var GPIO40HeaderV1 = []int{
	14, 15, 18, 23, 24, 25, 8, 7, 12, 16, 20, 21,
	2, 3, 4, 17, 27, 22, 10, 9, 11, 5, 6, 13, 19, 26,
}

var modelMaps = make(map[string]([]string))
var host *pi

// from http://www.raspberrypi-spy.co.uk/2012/09/checking-your-raspberry-pi-board-version/

// setModelMaps establishes the basic mapping between the P1 headers used, and the Pi hardware revisions that use them.
// setModelMaps is called from init()
func setModelMaps() {
	modelMaps["26v10"] = []string{"Beta", "0002", "0003"}
	modelMaps["26v20"] = []string{"0004", "0005", "0006", "0007", "0008", "0009",
		"000d", "000e", "000f"}
	modelMaps["40v10"] = []string{"0010", "0011", "0012", "0013", "0014", "0015",
		"a01040", "a01041", "a21041", "a22042", "900021",
		"900092", "900093", "920093", "a02082", "a22082"}
}

// findRevisionMap identifies which P1 pin header name to use based on the hardware revision
func findRevisionMap(revision string) string {
	for k, v := range modelMaps {
		for _, r := range v {
			if r == revision {
				return k
			}
		}
	}
	return ""
}

// getdetails allows only one system inspection to determine the current hardware profile
var getdetails sync.Once

// initOnce does the legwork for populating the system details
func initOnce() {
	model := readFilePanic(file(sys_model))
	revision := readRevision()
	host = buildPi(revision, model)
}

// readRevision gets the hardware revision for a RPi
func readRevision() string {
	cpuinfo := readFilePanic(file(proc_cpuinfo))
	revisionre := regexp.MustCompile(`(?sm).*^Revision\s+:\s+(\S+)\s*$.*`)
	revision := revisionre.ReplaceAllString(cpuinfo, "$1")
	return revision
}

var availableGPIO map[int]bool

func setAvailableGPIOs() {
	availableGPIO = make(map[int]bool)
	gpio := file(sys_gpio)
	nodes, err := ioutil.ReadDir(gpio)
	if err != nil {
		info("Unable to read folder %v: %v", gpio, err)
		return
	}
	// See sysfs standard, needs to be a base and ngpio file: https://www.kernel.org/doc/Documentation/gpio/sysfs.txt
	for _, f := range nodes {
		if strings.HasPrefix(f.Name(), "gpiochip") {
			chip := filepath.Join(gpio, f.Name())
			fbase := filepath.Join(chip, "base")
			fngpio := filepath.Join(chip, "ngpio")
			base, err := readStringFileAsInt(fbase)
			if err != nil {
				info("Unable to read file %v: %v", filepath.Join(chip, "base"), err)
				continue
			}
			ngpio, err := readStringFileAsInt(fngpio)
			if err != nil {
				info("Unable to read file %v: %v", filepath.Join(chip, "ngpio"), err)
				continue
			}
			for i := 0; i < ngpio; i++ {
				availableGPIO[base+i] = true
			}
		}
	}
}

func isChip(path string, name string) bool {
	if !strings.HasPrefix(name, "gpiochip") {
		return false
	}
	devname := filepath.Join(path, name, "device", "of_node", "name")
	s, err := os.Stat(devname)
	if err != nil || s.IsDir() {
		return false
	}
	contents, err := readFile(devname)
	if err != nil {
		return false
	}
	return contents == "gpio"
}

func buildPi(revision, model string) *pi {

	var pins []int
	pinMap := findRevisionMap(revision)
	def := "40V10"
	if pinMap == "" {
		log.Printf("Unable to locate an express mapping for revision '%v'. Using default %v'\n", revision, def)
		pinMap = def
	}
	switch pinMap {
	case "26v10":
		pins = GPIO26HeaderV1
	case "26v20":
		pins = GPIO26HeaderV2
	case "40v10":
		pins = GPIO40HeaderV1
	}

	sort.Ints(pins)

	return &pi{
		mu:        sync.Mutex{},
		model:     model,
		revision:  revision,
		gpiodir:   file(sys_gpio),
		gpioports: pins,
		portctrl:  make(map[int]*gport),
	}
}

// String produces a human readable representation of the Pi
func (p *pi) String() string {
	return fmt.Sprintf("Pi hardware revision %v and model %v with ports %v", p.revision, p.model, p.gpioports)
}

// Model returns the given name of the pi board
func (p *pi) Model() string {
	return p.model
}

// Revision returns the given board revision
func (p *pi) Revision() string {
	return p.revision
}

// P1GPIOPorts returns the possible set of P1 header GPIOPorts based on the pi board/revision.
// Note that some possible ports may be configured as a service other than GPIO (Uart, etc.)
func (p *pi) P1GPIOPorts() []int {
	cp := make([]int, len(p.gpioports))
	copy(cp, p.gpioports)
	return cp
}

// IsPiPort returns true if the specified port could be a GPIO Port on the pi P1 header
func (p *pi) IsP1Port(port int) bool {
	if port < 0 || port >= len(p.gpioports) {
		return false
	}
	for _, pt := range p.gpioports {
		if pt == port {
			return true
		}
	}
	return false
}

// GetPort returns a control point in to a GPIO Port.
// The control needs to be checked to ensure that the port is actually a GPIO Port
// as some ports may be multiplexed in to UARTs, I2C, etc. or the port may not exist.
func (p *pi) GetPort(port int) (GPIOPort, error) {
	if !availableGPIO[port] {
		return nil, fmt.Errorf("Port %v is not available on this system", port)
	}
	defer p.unlock(p.lock())
	pctrl, ok := p.portctrl[port]
	if ok {
		return pctrl, nil
	}
	pctrl = newGPIO(p, port)
	p.portctrl[port] = pctrl
	return pctrl, nil
}

func (p *pi) portFolder(port int) string {
	return file("sys", "class", "gpio", fmt.Sprintf("gpio%d", port))
}

func (p *pi) lock() bool {
	p.mu.Lock()
	return true
}

func (p *pi) unlock(bool) {
	p.mu.Unlock()
}
