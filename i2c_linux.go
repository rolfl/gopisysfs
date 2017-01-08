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

func I2CListDevices() ([]string, error) {
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

type I2CRecording struct {
	Timestamp time.Time
	Data      []byte
}

func copyBytes(buffer []byte, count int) []byte {
	ret := make([]byte, count)
	copy(ret, buffer)
	return ret
}

// I2CPoll establishes a connection to a slave I2C device and periodically reads a fixed number of bytes from that device.
// The dev, address, and bytes parameters indicates which device to read and how much to read each time.
// The bufferdepth determines how deep the returned channel's buffer is.
// All samples taken after the buffer is filled will be discarded until space is available.
// An unbuffered return is supported, and guarantees that a receive on that channel gets the most recent sample.
// The interval indicates the period to sample at.
// The returned channel will be closed if there's an error reading the device or the poller is closed using the returned termination function.
// Call the termination function returned when you no longer need to receive polling data.
func I2CPoll(dev string, address int, bytes int, bufferdepth int, interval time.Duration) (<-chan I2CRecording, func(), error) {

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

	// unbuffered channel - reader only gets data when asking to receive it, and they get the most recently available value.
	data := make(chan I2CRecording, 0)
	record := I2CRecording{time.Now(), copyBytes(buffer, n)}

	go func() {
		defer close(data)
		defer ctrl.Close()

		tick := time.NewTicker(interval)
		defer tick.Stop()

		var stamp time.Time

		// we do some nil channel tricks to manipulate the select statement. dest is part of that.
		dest := data

		for {
			select {
			case <-killer:
				return
			case dest <- record:
				// disable dest until there's a new record.
				dest = nil
			case stamp = <-tick.C:
				n, err := ctrl.Read(buffer)
				if err != nil {
					info("I2C Unexpected error reading %v: %v\n", dev, err)
					return
				}
				record = I2CRecording{stamp, copyBytes(buffer, n)}
				// indicate there's data to send and reenable dest.
				dest = data
			}
		}
	}()

	return data, termfn, nil

}
