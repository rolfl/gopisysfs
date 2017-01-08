package gopisysfs

import (
	"os"
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
	buff := make([]byte, 0, 10)

	timeout := 500 * time.Millisecond
	timeoutTs := unix.NsecToTimespec(int64(timeout))
	pollspec := []unix.PollFd{{Fd: int32(valf.Fd()), Events: unix.POLLIN | unix.POLLPRI | unix.POLLERR}}

	count := 0
	for {
		count++
		info("GPIO Monitor %v looping: %v\n", valf.Name(), count)
		select {
		case <-killer:
			// normal shut down
			return
		default:
			// keep on moving ... nothing to do here.
		}

		// wait up to some period for data to be there....
		state, err := unix.Ppoll(pollspec, &timeoutTs, nil)
		if err != nil {
			info("GPIO Monitor %v terminating: %v\n", valf.Name(), err)
			return
		}

		if state > 0 {

			// data to read....
			info("GPIO Monitor %v polled OK: %v\n", valf.Name(), state)

			stamp := time.Now()
			n, err := valf.Read(buff)
			if err != nil {
				info("GPIO Monitor %v terminating: %v\n", valf.Name(), err)
				return
			}
			// reset it for next read
			if _, err := valf.Seek(0, 0); err != nil {
				info("GPIO Monitor %v terminating: %v\n", valf.Name(), err)
				return
			}

			val := string(buff[:n]) == "1"
			event := Event{val, stamp}
			select {
			case data <- event:
				info("GPIO Monitor %v sending: %v\n", valf.Name(), event)
			case <-killer:
				// normal shut down
				return
			default:
				info("GPIO Monitor %v terminating: send receive channel overflow\n", valf.Name())
				return
			}
		} else {
			info("GPIO Monitor %v polled Timeout\n", valf.Name())
		}

	}

}

func buildMonitor(fname string, buffersize int) (<-chan Event, func(), error) {

	// open the value file, we will need the file descriptor
	valf, err := os.Open(p.value)
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
