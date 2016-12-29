package gopisysfs

import (
	"testing"
	"time"
)

func TestResetNoop(t *testing.T) {
	SetLogFn(t.Logf)
	pi := GetDetailsFor(testrevision, testmodel)
	t.Logf("Got details %v", pi)
	ch, err := pi.GPIOResetAsync(testinport, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	<-time.After(50 * time.Millisecond)
	select {
	case <-time.After(time.Second):
		t.Fatal("Expected to time out after 1 second, but it has been longer")
	case e := <-ch:
		if e != nil {
			t.Fatal(e)
		}
	}
}
