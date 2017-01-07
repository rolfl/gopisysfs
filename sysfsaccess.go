package gopisysfs

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	// pollInterval is how long we should wait between naieve polling of files
	pollInterval = 20 * time.Millisecond
	// forever is about 100 years.
	forever = 100 * 365 * 24 * time.Hour
)

var rootpath = "/"

// setRoot is designed to be called by the test cases to exercise some hard-to-change things on an actual pi.
func setRoot(rt string) {
	rootpath = rt
}

// file gets a file path inside the /sys file system,
// but it can be hooked by the test cases to use a test filesystem instead of the real /sys
func file(paths ...string) string {
	path := filepath.Join(paths...)
	if !filepath.IsAbs(path) {
		path = filepath.Join(rootpath, path)
	}
	return path
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
	// intervals at every poll cycle
	interval := time.NewTicker(pollInterval).C
	// naieve polling system
	go func() {
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

// awaitFileRemove establishes an asynchronous poll on a file location until it is removed
// at which point the returned channel will return a nil on the channel. A non-nil indicates
// an error in the polling.
func awaitFileRemove(name string, timeout time.Duration) (<-chan error, error) {

	ret := make(chan error, 1)

	// file is not there. Easy.
	if !checkFile(name) {
		ret <- nil
		return ret, nil
	}

	// set up notification and timeout
	tout := time.After(timeout)
	// intervals at every 20 milliseconds
	interval := time.NewTicker(pollInterval).C

	// naieve polling system
	go func() {
		for {

			if !checkFile(name) {
				// gone!
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

func readStringFileAsInt(name string) (int, error) {
	data, err := readFile(name)
	if err != nil {
		return 0, err
	}
	val, err := strconv.Atoi(data)
	if err != nil {
		return 0, fmt.Errorf("Unable to convert value %v from %v to an int: %v", data, name, err)
	}
	return val, nil
}

//readFile reads the file and returns the contents as a string (trimmed)
func readFile(name string) (string, error) {
	data, err := ioutil.ReadFile(name)
	if err != nil {
		return "", err
	}
	str := string(data)
	str = strings.TrimSpace(str)
	return str, nil
}

// readBuffer reads a file in to a byte buffer
func readBytes(name string) ([]byte, error) {
	return ioutil.ReadFile(name)
}

// writeBuffer writes a buffer in to a file
func writeBuffer(name string, data []byte) error {
	return ioutil.WriteFile(name, data, 0444)
}

// writeFile will overwrite the specified file with the given string content
func writeFile(name, text string) error {
	data := []byte(text)
	return ioutil.WriteFile(name, data, 0444)
}

func checkWritable(name string) bool {
	if stat, err := os.Stat(name); err == nil {
		// exists, but is it writable?
		mode := os.O_RDWR
		desc := "writable file"
		if stat.IsDir() {
			mode = os.O_RDONLY
			desc = "readable folder"
		}
		// Note, you can open directories as well
		file, err := os.OpenFile(name, mode, 0)
		if err != nil {
			fmt.Printf("Existing file %v but it is not a %v: %v\n", name, desc, err)
			return false
		}
		file.Close()
		// already exists
		return true
	}
	return false
}

// checkFile retuns true if the specified file exists
func checkFile(name string) bool {
	if _, err := os.Stat(name); err == nil {
		// exists, but is it writable?
		// already exists
		return true
	}
	return false
}
