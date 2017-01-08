package gopisysfs

import (
	"fmt"
)

func buildMonitor(fname string, buffersize int) (<-chan Event, func(), error) {
	return nil, nil, fmt.Errorf("Do not support setupMonitor on windows")
}
