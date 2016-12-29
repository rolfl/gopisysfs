package gopisysfs

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

const (
	sys_model    = "sys/firmware/devicetree/base/model"
	proc_cpuinfo = "proc/cpuinfo"
)

var rootpath = "/"

func setRoot(rt string) {
	rootpath = rt
}

func file(paths ...string) string {
	path := filepath.Join(paths...)
	if !filepath.IsAbs(path) {
		path = filepath.Join(rootpath, path)
	}
	return path
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

//Pi contains information describing the Pi model we are running on
type Pi struct {
	Model     string
	Revision  string
	GPIOPorts []int
}

type LogFunction func(format string, args ...interface{})

var logfn LogFunction

func SetLogFn(lfn LogFunction) {
	logfn = lfn
}

func info(format string, args ...interface{}) {
	lfn := logfn
	if lfn == nil {
		return
	}
	lfn(format, args...)
}

var syslock sync.Mutex

var modelMaps = make(map[string]([]string))
var details Pi
var revisionre = regexp.MustCompile(`(?sm).*^Revision\s+:\s+(\S+)\s*$.*`)

// from http://www.raspberrypi-spy.co.uk/2012/09/checking-your-raspberry-pi-board-version/

// init establishes the basic mapping between the P1 headers used, and the Pi hardware revisions that use them
func init() {
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
	details = GetDetailsFor(revision, model)
}

// GetDetails returns the details of the Pi that is currently being run on
func GetPi() Pi {
	getdetails.Do(initOnce)
	return details
}

// GetDetailsFor returns the Pi internal details given a specific model and hardware revision
func GetDetailsFor(revision, model string) Pi {

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

	return Pi{
		Model:     model,
		Revision:  revision,
		GPIOPorts: pins,
	}
}

func (pi *Pi) String() string {
	return fmt.Sprintf("Pi hardware revision %v and model %v with ports %v", pi.Revision, pi.Model, pi.GPIOPorts)
}

// readRevision gets the hardware revision for a RPi
func readRevision() string {
	cpuinfo := readFilePanic(file(proc_cpuinfo))
	revision := revisionre.ReplaceAllString(cpuinfo, "$1")
	return revision
}

// readFilePanic reads a file returning the contents as a string, and panics if it cannot be read
func readFilePanic(name string) string {
	data, err := readFile(name)
	if err != nil {
		log.Panicf("Unable to read file %v: %v", name, err)
	}
	return data
}

// awaitFileCreate establishes an asynchronous poll on a file location until it exists
// at which point the returned channel will return a nil on the channel. A non-nil indicates
// an error in the polling.
func awaitFileCreate(name string, timeout time.Duration) (<-chan error, error) {

	ret := make(chan error, 1)

	if checkFile(name) {
		ret <- nil
		return ret, nil
	}

	dir := filepath.Dir(name)
	if stat, err := os.Stat(dir); err != nil || !stat.IsDir() {
		if err != nil {
			return nil, fmt.Errorf("Unable to poll for a file in a nonexistent folder %v: %v", dir, err)
		}
		return nil, fmt.Errorf("Unable to poll for a file in a non-folder %v: %v", dir, stat)
	}

	// set up notification and timeout
	tout := time.After(timeout)
	// intervals at every 20 milliseconds
	interval := time.NewTicker(20 * time.Millisecond).C
	// naieve polling system
	go func() {
		// wait for events on the folder to indicate availability of the file
		for {

			if checkFile(name) {
				// Found it!
				ret <- nil
				return
			}

			select {
			case <-tout:
				ret <- fmt.Errorf("Timed out waiting for %v after %v", name, timeout)
				return
			case <-interval:
				// ignore specific event, check actual file later
			}
		}
	}()

	return ret, nil

}

// lock and the matching unlock function ensure that all IO from this program to the sysfs is serialized.
func lock() bool {
	syslock.Lock()
	return true
}

// unlock and the matching lock function ensure that all IO from this program to the sysfs is serialized.
func unlock(bool) {
	syslock.Unlock()
}

//readFile reads the file and returns the contents as a string (trimmed)
func readFile(name string) (string, error) {
	defer unlock(lock())
	data, err := ioutil.ReadFile(name)
	if err != nil {
		return "", err
	}
	str := string(data)
	str = strings.TrimSpace(str)
	return str, nil
}

// writeFile will overwrite the specified file with the given string content
func writeFile(name, text string) error {
	defer unlock(lock())
	data := []byte(text)
	return ioutil.WriteFile(name, data, 0444)
}

// checkFile retuns true if the specified file exists
func checkFile(name string) bool {
	defer unlock(lock())
	if _, err := os.Stat(name); err == nil {
		// already exists
		return true
	}
	return false
}
