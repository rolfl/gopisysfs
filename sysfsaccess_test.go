package gopisysfs

import (
	"fmt"
	"os"
	"testing"
	"time"
)

func TestModel(t *testing.T) {
	t.Log("Testing details")
	model := readFilePanic(sys_model)
	if model == "" {
		t.Errorf("Unable to get model")
	}
	revision := readRevision()
	if revision == "" {
		t.Errorf("Unable to get revision")
	}

	t.Logf("Got Got model %v and revision %v", model, revision)
	
}

func TestWriteReadFile(t *testing.T) {
	name := fmt.Sprintf("/tmp/gopitest.%v.readwrite", os.Getpid())
	err := writeFile(name, "boo")
	if err != nil {
		t.Fatal(err)
	}

	val, err := readFile(name)
	if err != nil {
		t.Fatal(err)
	}
	if val != "boo" {
		t.Errorf("Expected to read '%v' but got '%v'", "boo", val)
	}

}

func TestAwaitFileExists(t *testing.T) {
	SetLogFn(t.Logf)
	name := fmt.Sprintf("/tmp/gopitest.%v.exists", os.Getpid())
	t.Logf("Using test file %v", name)
	err := writeFile(name, "boo")
	if err != nil {
		t.Fatal(err)
	}
	ch, err := awaitFileCreate(name, 2 * time.Second)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("About to wait on channel\n")
	err, ok := <-ch
	t.Logf("Got notify on channel (closed %v): %v\n", !ok, err)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("Channel incorrectly closed without a value")
	}

	t.Logf("Checking file contents\n")
	data, err := readFile(name)
	if data != "boo" {
		t.Fatalf("Expected to read boo but got: %v", data)
	}

}

func TestAwaitFile(t *testing.T) {
	SetLogFn(t.Logf)
	name := fmt.Sprintf("/tmp/gopitest.%v.await", os.Getpid())
	t.Logf("Using test file %v", name)
	ch, err := awaitFileCreate(name, 2 * time.Second)
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		<-time.After(200 * time.Millisecond)
		t.Logf("About to write %v\n", name)
		writeFile(name, "boo")
		t.Logf("Wrote %v\n", name)
	}()
	t.Logf("About to wait on channel\n")
	err, ok := <-ch
	t.Logf("Got notify on channel (closed %v): %v\n", !ok, err)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("Channel incorrectly closed without a value")
	}

	t.Logf("Checking file contents\n")
	data, err := readFile(name)
	if data != "boo" {
		t.Fatalf("Expected to read boo but got: %v", data)
	}

}
