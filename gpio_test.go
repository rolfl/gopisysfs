package gopisysfs

import (
	"testing"
)

func TestResetNoop(t *testing.T) {
	//mustbereal()
	SetLogFn(t.Logf)
	pi := GetDetailsFor(testrevision, testmodel)
	t.Logf("Got details %v", pi)
	port, err := pi.GetPort(testinport)
	if err != nil {
		t.Fatal(err)
	}
	err = port.Reset()
	if err != nil {
		t.Fatal(err)
	}
}
