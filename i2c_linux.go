package gopisysfs

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

const (
	sys_i2c   = "sys/class/i2c-dev"
	i2c_SLAVE = 0x703
)

func ListI2CDevs() ([]string, error) {
	devdir := file(sys_i2c)
	files, err := ioutil.ReadDir(devdir)
	if err != nil {
		return nil, err
	}

	names := []string{}
	for _, f := range files {
		if f.IsDir() {
			continue
		}
		name := f.Name()
		dev := filepath.Join("/dev", name)
		info("I2C Checking %v\n", dev)
		if _, err := os.Stat(dev); err != nil {
			continue
		}
		names = append(names, dev)
	}
	return names, nil
}

type Recording struct {
	Timestamp time.Time
	Data      []byte
}

func copyBytes(buffer []byte, count int) []byte {
	ret := make([]byte, count)
	copy(ret, buffer)
	return ret
}

func PollI2C(dev string, address int, bytes int, interval time.Duration) (<-chan Recording, func(), error) {

	ctrl, err := os.OpenFile(dev, os.O_RDWR, 0)
	if err != nil {
		return nil, nil, err
	}

	killer := make(chan bool, 1)

	termfn := func() {
		killer <- true
	}

	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(ctrl.Fd()), i2c_SLAVE, uintptr(address))
	if errno != 0 {
		return nil, nil, errno
	}

	buffer := make([]byte, bytes)
	n, err := ctrl.Read(buffer)
	if err != nil {
		ctrl.Close()
		return nil, nil, err
	}

	// single slot channel
	data := make(chan Recording, 1)

	data <- Recording{time.Now(), copyBytes(buffer, n)}

	go func() {
		defer close(data)
		defer ctrl.Close()

		tick := time.NewTicker(interval)
		defer tick.Stop()

		var stamp time.Time

		for {
			select {
			case <-killer:
				return
			case stamp = <-tick.C:
			}
			n, err := ctrl.Read(buffer)
			if err != nil {
				info("I2C Unexpected error reading %v: %v\n", dev, err)
				return
			}
			data <- Recording{stamp, copyBytes(buffer, n)}
		}
	}()

	return data, termfn, nil

}
