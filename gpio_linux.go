package gopisysfs

import (
	"os"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

func monitorData(valf *os.File, data chan<- Event, killer <-chan bool) {

	// This is run inside a goroutine

	defer func() {
		info("GPIO Monitor %v killing\n", valf.Name())
		close(data)
		valf.Close()
	}()

	// create a buffer to read the values in to.
	buff := make([]byte, 10)

	timeout := 500
	pollflag := int16(unix.POLLPRI | unix.POLLERR)
	fd := int32(valf.Fd())

	ready := true

	for {

		if ready {
			stamp := time.Now()
			ready = false

			// reset it for read
			if _, err := valf.Seek(0, 0); err != nil {
				info("GPIO Monitor %v terminating: %v\n", valf.Name(), err)
				return
			}

			n, err := valf.Read(buff)
			if err != nil {
				info("GPIO Monitor %v terminating: %v\n", valf.Name(), err)
				return
			}
			got := strings.TrimSpace(string(buff[:n]))
			val := got == "1"
			event := Event{val, stamp}
			select {
			case data <- event:
			case <-killer:
				// normal shut down
				return
			default:
				info("GPIO Monitor %v terminating: send receive channel overflow\n", valf.Name())
				return
			}
		}

		select {
		case <-killer:
			// normal shut down
			return
		default:
			// keep on moving ... nothing to do here.
		}

		// wait up to some period for data to be there....
		pollspec := []unix.PollFd{{Fd: fd, Events: pollflag}}
		state, err := unix.Poll(pollspec, timeout)
		if err != nil {
			info("GPIO Monitor %v terminating: %v\n", valf.Name(), err)
			return
		}

		if state > 0 {
			// data to read....
			ready = true
		}

	}

}

func buildMonitor(fname string, buffersize int) (<-chan Event, func(), error) {

	// open the value file, we will need the file descriptor
	valf, err := os.Open(fname)
	if err != nil {
		return nil, nil, err
	}

	killer := make(chan bool, 1)
	killfn := func() {
		select {
		case killer <- true:
		default:
		}
	}

	data := make(chan Event, buffersize)

	go monitorData(valf, data, killer)

	return data, killfn, nil

}
