package gopisysfs

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

var nowtime string

func init() {
	abs, _ := filepath.Abs("testdata")
	setRoot(abs)
	nowtime = fmt.Sprintf("%v", time.Now().UnixNano())
}

func tmpFile(ext string) string {
	return file("tmp", fmt.Sprintf("gopitest.%v.%v.%v", os.Getpid(), nowtime, ext))
}

func TestCheck(t *testing.T) {
	name := tmpFile("checkfile")
	if checkFile(name) {
		t.Errorf("Expected file %v to not exist, but it does", name)
	}
	writeFile(name, "boo")
	if !checkFile(name) {
		t.Errorf("Expected file %v to exist, but it does not", name)
	}
}

func TestModel(t *testing.T) {
	t.Log("Testing details")
	model := readFilePanic(file(sys_model))
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
	name := tmpFile("readwrite")
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
	name := tmpFile("awaitpre")
	t.Logf("Using test file %v", name)
	err := writeFile(name, "boo")
	if err != nil {
		t.Fatal(err)
	}
	ch, err := awaitFileCreate(name, 2*time.Second)
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
	name := tmpFile("awaitpost")
	t.Logf("Using test file %v", name)
	ch, err := awaitFileCreate(name, 2*time.Second)
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
