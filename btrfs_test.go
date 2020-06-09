package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"
)

const mount = "/tmp/btrfs"

func writeFile(t *testing.T, name string, data []byte) string {
	os.Mkdir(fmt.Sprintf("%s/%s", mount, t.Name()), 0777)
	path := fmt.Sprintf("%s/%s/%s", mount, t.Name(), name)
	err := ioutil.WriteFile(path, data, 0644)
	if err != nil {
		t.Fatal("Failed to write test file: ", err)
	}
	return path
}

func TestDedupeDuplicateFiles(t *testing.T) {
	data := []byte("foo")
	a := writeFile(t, "a", data)
	b := writeFile(t, "b", data)
	err := DedupeFiles([]string{a, b})
	if err != nil {
		t.Fatal(err)
	}
}

func TestDedupeDissimilarFiles(t *testing.T) {
	aData := []byte("foo")
	bData := []byte("bar")
	a := writeFile(t, "a", aData)
	b := writeFile(t, "b", bData)
	err := DedupeFiles([]string{a, b})
	if err != nil {
		t.Fatal(err)
	}
}
