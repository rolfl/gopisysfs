package gopisysfs

import (
	"testing"
)

func TestResetNoop(t *testing.T) {
	mustbereal()
	SetLogFn(t.Logf)
	pi := GetDetailsFor(testrevision, testmodel)
	t.Logf("Got details %v", pi)
	port := pi.GetPort(testinport)
	if !port.IsGPIO() {
		t.Fatalf("Port %v is not a GPIO port. Can't test", testinport)
	}
	err := port.Reset()
	if err != nil {
		t.Fatal(err)
	}
}
