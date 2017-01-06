package gopisysfs

import ()

const testrevision = "a22082"
const testmodel = "Raspberry Pi 3 Model B Rev 1.2"
const testoutport = 23
const testinport = 24

func mustbereal() {
	if !IsOnPi() {
		panic("Must be running on real hardware")
	}
}
