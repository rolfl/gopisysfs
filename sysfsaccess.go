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

	"github.com/fsnotify/fsnotify"
)

const (
	sys_model    = "/sys/firmware/devicetree/base/model"
	proc_cpuinfo = "/proc/cpuinfo"
)

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

//PiDetails contains information describing the Pi model we are running on
type PiDetails struct {
	Model     string
	Revision  string
	GPIOPorts []int
}

var logfn func(format string, args ...interface{})

func SetLogFn(lfn func(string, ...interface{})) {
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
var details PiDetails
var revisionre = regexp.MustCompile(`(?sm).*^Revision\s+:\s+(\S+)\s*$.*`)

// from http://www.raspberrypi-spy.co.uk/2012/09/checking-your-raspberry-pi-board-version/

func init() {
	modelMaps["26v10"] = []string{"Beta", "0002", "0003"}
	modelMaps["26v20"] = []string{"0004", "0005", "0006", "0007", "0008", "0009",
		"000d", "000e", "000f"}
	modelMaps["40v10"] = []string{"0010", "0011", "0012", "0013", "0014", "0015",
		"a01040", "a01041", "a21041", "a22042", "900021",
		"900092", "900093", "920093", "a02082", "a22082"}
}

func initOnce() {
	model := readFilePanic(sys_model)
	revision := readRevision()
	details = GetDetailsFor(revision, model)
}

var getdetails sync.Once

// GetDetails returns the details of the Pi that is currently being run on
func GetDetails() PiDetails {

	getdetails.Do(initOnce)

	return details
}

func GetDetailsFor(revision, model string) PiDetails {

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
	case "40V10":
		pins = GPIO40HeaderV1
	}

	return PiDetails{
		Model:     model,
		Revision:  revision,
		GPIOPorts: pins,
	}
}

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

// readRevision gets the hardware revision for a RPi
func readRevision() string {
	cpuinfo := readFilePanic(proc_cpuinfo)
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

//readFile reads the file and returns the contents as a string
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

func writeFile(name, text string) error {
	defer unlock(lock())
	data := []byte(text)
	return ioutil.WriteFile(name, data, 0444)
}

func awaitFileCreate(name string, timeout time.Duration) (<-chan error, error) {

	ret := make(chan error, 1)
	if _, err := os.Stat(name); err == nil {
		// already exists
		ret <- nil
		return ret, nil
	}

	// set up notification and timeout

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	dir := filepath.Dir(name)

	err = watcher.Add(dir)
	if err != nil {
		return nil, fmt.Errorf("Unable to create watcher on %v: %v", name, err)
	}

	tout := time.After(timeout)

	go func() {
		// wait for events on the folder to indicate availability of the file
		defer func() {
			close(ret)
			watcher.Close()
		}()
		for {
			_, serr := os.Stat(name)
			if serr == nil {
				ret <- nil
				return
			}
			select {
			case <-tout:
				ret <- fmt.Errorf("Timed out waiting for %v after %v", name, timeout)
				return
			case err, ok := <-watcher.Errors:
				if !ok {
					err = fmt.Errorf("Watcher Errors channel closed too soon watching %v", dir)
				}
				ret <- err
				return
			case _, ok := <-watcher.Events:
				if !ok {
					ret <- fmt.Errorf("Unexpected early close of Watcher.Events watching %v", dir)
					return
				}
				// ignore specific event, check actual file later
			}
		}
	}()

	return ret, nil

}

func lock() bool {
	syslock.Lock()
	return true
}

func unlock(bool) {
	syslock.Unlock()
}
