package lotf

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFileLines(t *testing.T) {
	// prepare
	dir, err := ioutil.TempDir("", "lotf")
	if err != nil {
		t.Fatalf("TempDir failed: %s", err)
	}
	t.Logf("tmpdir: %s", dir)
	defer os.RemoveAll(dir)
	testFile, err := os.OpenFile(filepath.Join(dir, "TestFileLines.testfile"), os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		t.Fatalf("testFile failed: %s", err)
	}

	// 4 empty lines
	if err = testFile.Truncate(0); err != nil {
		t.Fatalf("truncate testFile failed: %s", err)
	}
	if _, err = testFile.Seek(0, os.SEEK_SET); err != nil {
		t.Fatalf("seek testFile failed: %s", err)
	}
	if _, err = testFile.WriteString("\n\n\n\n"); err != nil {
		t.Fatalf("write testFile failed: %s", err)
	}

	//  0 1 2 3
	// "\n\n\n\n"

	// tail -n0
	pos, err := FileLines(testFile, 0)
	if err != nil {
		t.Fatalf("should returns error")
	}

	// tail -n1
	if pos, err = FileLines(testFile, 1); err != nil {
		t.Fatalf("FileLines failed: %s", err)
	}
	if pos != 3 {
		t.Fatalf("tail 1 should return: 3, but: %d", pos)
	}

	// tail -n3
	if pos, err = FileLines(testFile, 3); err != nil {
		t.Fatalf("FileLines failed: %s", err)
	}
	if pos != 1 {
		t.Fatalf("tail 3 should return: 1, but: %d", pos)
	}

	// tail -n4
	if pos, err = FileLines(testFile, 4); err != nil {
		t.Fatalf("FileLines failed: %s", err)
	}
	if pos != 0 {
		t.Fatalf("tail 4 should return: 0, but: %d", pos)
	}

	// tail -n10
	if pos, err = FileLines(testFile, 10); err != nil {
		t.Fatalf("FileLines failed: %s", err)
	}
	if pos != 0 {
		t.Fatalf("tail 10 should return: 0, but: %d", pos)
	}


	// 4 empty lines, ended with no newline spaces
	if err = testFile.Truncate(0); err != nil {
		t.Fatal("truncate testFile failed: %s", err)
	}
	if _, err = testFile.Seek(0, os.SEEK_SET); err != nil {
		t.Fatalf("seek testFile failed: %s", err)
	}
	if _, err = testFile.WriteString("\n\n\n\n   "); err != nil {
		t.Fatalf("write testFile failed: %s", err)
	}
	// tail -n1
	if pos, err = FileLines(testFile, 1); err != nil {
		t.Fatalf("FileLines failed: %s", err)
	}
	if pos != 3 {
		t.Fatalf("tail 1 should return: 3, but: %d", pos)
	}

	// file filled with "a" in BUSIZE - 1 plus 4 empty lines
	if err = testFile.Truncate(0); err != nil {
		t.Fatal("truncate testFile failed: %s", err)
	}
	if _, err = testFile.Seek(0, os.SEEK_SET); err != nil {
		t.Fatalf("seek testFile failed: %s", err)
	}
	s := strings.Repeat("a", BUFSIZ - 1) + "\n\n\n\n"
	if _, err = testFile.WriteString(s); err != nil {
		t.Fatalf("write testFile failed: %s", err)
	}
	if pos, err = FileLines(testFile, 3); err != nil {
		t.Fatalf("FileLines failed: %s", err)
	}
	if pos != BUFSIZ {
		t.Fatalf("tail 1 should return: %d, but: %d", BUFSIZ, pos)
	}
}

/*
func TestFileLinesFilter(t *testing.T) {
	// prepare
	dir, err := ioutil.TempDir("", "lotf")
	if err != nil {
		t.Fatalf("TempDir failed: %s", err)
	}
	t.Logf("tmpdir: %s", dir)
	defer os.RemoveAll(dir)
	testFile, err := os.OpenFile(filepath.Join(dir, "TestFileLines.testfile"), os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		t.Fatalf("testFile failed: %s", err)
	}
	// 4 empty lines and 3 non-empty lines
	if err = testFile.Truncate(0); err != nil {
		t.Fatalf("truncate testFile failed: %s", err)
	}
	if _, err = testFile.Seek(0, os.SEEK_SET); err != nil {
		t.Fatalf("seek testFile failed: %s", err)
	}
	if _, err = testFile.WriteString("\n\na\nb\n\nc\nd"); err != nil {
		t.Fatalf("write testFile failed: %s", err)
	}
	// create filter
	filter, err := CreateReFilter("testfilter")
	if err != nil {
		t.Fatalf("failed to create filter: %s", err)
	}

	//  0 1 23 45 6 78 9
	// "\n\na\nb\n\nc\nd"

	// tail -n1
	pos, err := FileLines(testFile, 1) // regexp "^$"
	if err != nil {
		t.Fatalf("should returns error")
	}
	if pos != 6 {
		t.Fatalf("tail 1 should returns: 6, but: %d", pos)
	}
}
*/
func TestTailWatcher(t *testing.T) {
	// prepare
	dir, err := ioutil.TempDir("", "lotf")
	if err != nil {
		t.Fatalf("TempDir failed: %s", err)
	}
	t.Logf("tmpdir: %s", dir)
	defer os.RemoveAll(dir)
	for i := 0; i < 10; i++ {
		testFile, err := os.OpenFile(filepath.Join(dir, fmt.Sprintf("TailWatcher.%d", i)),
			os.O_RDWR|os.O_CREATE, 0666)
		if err != nil {
			t.Fatalf("failed to create testFile: %s", err)
		}
		if _, err := testFile.WriteString("test"); err != nil {
			t.Fatalf("failed to WriteString to testFile: %s", err)
		}
		if err := testFile.Close(); err != nil {
			t.Fatalf("failed to close testFile: %s", err)
		}
	}

	// constructor
	tw, err := NewTailWatcher()
	if err != nil {
		t.Fatalf("could not create TailWatcher: %s", err)
	}
	defer tw.Close()

	// error handling
	go func() {
		for err := range tw.Error {
			t.Fatalf("error received: %s", err)
		}
	}()

	// Add and Remove one file
	if _, err := tw.Add(filepath.Join(dir, "TailWatcher.1"), 5, nil, 5); err != nil {
		t.Fatalf("failed to Add to TailWatcher: %s", err)
	}

	if _, err = tw.Add(filepath.Join(dir, "TailWatcher.1"), 5, nil, 5); err == nil {
		t.Fatal("successed to Add duplicate name to TailWatcher")
	}

	if err = tw.Remove(filepath.Join(dir, "TailWatcher.1")); err != nil {
		t.Fatalf("failed to Remove from TailWatcher: %s", err)
	}

	if err = tw.Remove(filepath.Join(dir, "TailWatcher.1")); err == nil {
		t.Fatal("removing unwatched file returns no error")
	}

	if err = tw.Close(); err != nil {
		t.Fatalf("failed to Close TailWatcher: %s", err)
	}
}

func TestTailAdd(t *testing.T) {
	// prepare
	dir, err := ioutil.TempDir("", "lotf")
	if err != nil {
		t.Fatalf("TempDir failed: %s", err)
	}
	t.Logf("tmpdir: %s", dir)
	defer os.RemoveAll(dir)
	var testFiles [10]*os.File

	for i := 0; i < 10; i++ {
		testFiles[i], err = os.OpenFile(filepath.Join(dir, fmt.Sprintf("TailWatcher.%d", i)),
			os.O_RDWR|os.O_CREATE, 0666)
		if err != nil {
			t.Fatalf("failed to create testFile: %s", err)
		}
		defer testFiles[i].Close()
	}

	// constructor
	tw, err := NewTailWatcher()
	if err != nil {
		t.Fatalf("could not create TailWatcher: %s", err)
	}
	defer tw.Close()

	// error handling
	go func() {
		for err := range tw.Error {
			t.Logf("error received: %v", err)
		}
	}()

	// Add and Remove one file
	tail, err := tw.Add(filepath.Join(dir, "TailWatcher.1"), 5, nil, 5)
	if  err != nil {
		t.Fatalf("failed to Add to TailWatcher: %s", err)
	}

	tailch := make(chan *string)
	done := make(chan bool)
	var line *string

	go func() {
		for {
			p := tail.Next()
			if p == nil { break }
			tailch <- p
		}
		done <- true
	}()

	// just Blocking
	select {
	case line = <-tailch:
		t.Fatal("failed to block, but got: %s", *line)
	case <-time.After(1 * time.Second):
	}

	// containing non LF terminated string
	go func() { testFiles[1].WriteString("test string\nnot-LF-terminated") }()
	select {
	case line = <-tailch:
	case <-time.After(1 * time.Second):
		t.Fatal("failed to get next 1")
	}
	if *line != "test string" {
		t.Fatal("expect `test string' but got: %s", *line)
	}

	// just add LF before prev string
	go func() { testFiles[1].WriteString("\n") }()
	select {
	case line = <-tailch:
	case <-time.After(1 * time.Second):
		t.Fatal("failed to get next 2")
	}
	if *line != "not-LF-terminated" {
		t.Fatalf("expect `not-LF-terminated' but got: %s", *line)
	}

	// remove, finish goroutine
	if err = tw.Remove(filepath.Join(dir, "TailWatcher.1")); err != nil {
		t.Fatalf("failed to Remove from TailWatcher: %s", err)
	}
	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("Next() is still blocking")
	}

	if err = tw.Close(); err != nil {
		t.Fatalf("failed to Close TailWatcher: %s", err)
	}
}

func TestTailRemoveCreate(t *testing.T) {
	// prepare
	dir, err := ioutil.TempDir("", "lotf")
	if err != nil {
		t.Fatalf("TempDir failed: %s", err)
	}
	t.Logf("tmpdir: %s", dir)
	defer os.RemoveAll(dir)
	fname := filepath.Join(dir, "TailWatcher.testfile")
	tmpfname := filepath.Join(dir, "TailWatcher.tmpfile")

	testFile, err := os.OpenFile(fname, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		t.Fatalf("failed to create testFile: %s", err)
	}
	// file contains 6 lines - a b c d e f
	if _, err = testFile.WriteString("ABCDEFGHIJKLMNOPQRSTUVWXYZa\nb\nc\nd\ne\nf\n"); err != nil {
		t.Fatalf("write testFile failed: %s", err)
	}
	testFile.Close()

	// constructor
	tw, err := NewTailWatcher()
	if err != nil {
		t.Fatalf("could not create TailWatcher: %s", err)
	}
	defer tw.Close()

	// error handling
	go func() {
		for err := range tw.Error {
			t.Fatalf("error received: %s", err)
		}
	}()
	tail, err := tw.Add(fname, 5, nil, 5)
	if err != nil {
		t.Fatalf("failed to Add to TailWatcher: %v", err)
	}

	var s string
	for i := 0; i < 5; i++ {
		s += *tail.Next()
	}
	if s != "bcdef" {
		t.Fatalf("expect bcdef but got: %s\n", s)
	}

	// rename
	if err := os.Rename(fname, tmpfname); err != nil {
		t.Fatalf("failed to rename testfile: %s", err)
	}
	// and recreate with lines - 1 2 3 4 5 6
	testFile, err = os.OpenFile(fname, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		t.Fatalf("failed to create testFile: %s", err)
	}
	if _, err = testFile.WriteString("1\n2\n3\n4\n5\n6\n"); err != nil {
		t.Fatalf("write testFile failed: %s", err)
	}
	defer testFile.Close()

	for i := 0; i < 6; i++ {
		s += *tail.Next()
	}
	if s != "bcdef123456" {
		t.Fatalf("expect bcdef123456 but got: %s\n", s)
	}

	if _, err = testFile.WriteString("7\n"); err != nil {
		t.Fatalf("write testFile failed: %s", err)
	}
	if *tail.Next() != "7" {
		t.Fatalf("expect 7 but got: %s\n", s)
	}
}

func TestFilesInSameDir(t *testing.T) {
	// prepare
	dir, err := ioutil.TempDir("", "lotf")
	if err != nil {
		t.Fatalf("TempDir failed: %s", err)
	}
	t.Logf("tmpdir: %s", dir)
	defer os.RemoveAll(dir)
	// create 10 files in a same dir
	var testFiles [10]*os.File
	for i := 0; i < 10; i++ {
		testFiles[i], err = os.OpenFile(filepath.Join(dir, fmt.Sprintf("TailWatcher.%d", i)),
			os.O_RDWR|os.O_CREATE, 0666)
		if err != nil {
			t.Fatalf("failed to create testFile: %s", err)
		}
		if _, err := testFiles[i].WriteString("test"); err != nil {
			t.Fatalf("failed to WriteString to testFile: %s", err)
		}
	}
	defer func() {
		for i := 2; i < 10; i++ {
			if err := testFiles[i].Close(); err != nil {
				t.Fatalf("failed to close testFile: %s", err)
			}
		}
	}()

	// constructor
	tw, err := NewTailWatcher()
	if err != nil {
		t.Fatalf("could not create TailWatcher: %s", err)
	}
	defer tw.Close()

	// error handling
	go func() {
		for err := range tw.Error {
			t.Fatalf("error received: %s", err)
		}
	}()

	// watch 10 files in a same dir
	var tails [10]Tail
	for i := 0; i < 10; i++ {
		fname := filepath.Join(dir, fmt.Sprintf("TailWatcher.%d", i))
		if tails[i], err = tw.Add(fname, 5, nil, 5); err != nil {
			t.Fatalf("failed to Add to TailWatcher: %s", err)
		}
		if err = tw.Remove(fname); err != nil {
			t.Fatalf("failed to Remove from TailWatcher: %s", err)
		}
		if tails[i], err = tw.Add(fname, 5, nil, 5); err != nil {
			t.Fatalf("failed to Add to TailWatcher: %s", err)
		}
	}

	if _, err = testFiles[0].WriteString("TEST0\ntest0"); err != nil {
		t.Fatalf("write testFile failed: %s", err)
	}
	if _, err = testFiles[1].WriteString("TEST1\ntest1"); err != nil {
		t.Fatalf("write testFile failed: %s", err)
	}
	if _, err = testFiles[2].WriteString("TEST2\ntest2"); err != nil {
		t.Fatalf("write testFile failed: %s", err)
	}
	tailch := make(chan *string)
	var line *string
	go func() {
		tailch <- tails[2].Next()
	}()
	select {
	case line = <-tailch:
		if *line != "TEST2" {
			t.Fatal("expect string TEST2, but got: %s", line)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timed out. no line returned")
	}

	if *(tails[0].Next()) != "TEST0" {
			t.Fatal("expect string TEST0, but got: %s", line)
	}
	if *(tails[1].Next()) != "TEST1" {
			t.Fatal("expect string TEST0, but got: %s", line)
	}

	// delete file and read the rest
	if err = testFiles[0].Close(); err != nil {
		t.Fatalf("failed to close testFile: %s", err)
	}
	if err = os.Remove(testFiles[0].Name()); err != nil {
		t.Fatalf("failed to remove testFile: %s", err)
	}
	if *(tails[0].Next()) != "test0" {
		t.Fatal("expect string TEST0, but got: %s", line)
	}
	if err = testFiles[1].Close(); err != nil {
		t.Fatalf("failed to close testFile: %s", err)
	}
	if err = os.Remove(testFiles[1].Name()); err != nil {
		t.Fatalf("failed to remove testFile: %s", err)
	}
	if *(tails[1].Next()) != "test1" {
		t.Fatal("expect string TEST0, but got: %s", line)
	}
}

func TestLookup(t *testing.T) {
	// prepare
	dir, err := ioutil.TempDir("", "lotf")
	if err != nil {
		t.Fatalf("TempDir failed: %s", err)
	}
	t.Logf("tmpdir: %s", dir)
	defer os.RemoveAll(dir)
	fname := filepath.Join(dir, "TailWatcher.testfile")
	testFile, err := os.OpenFile(fname, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		t.Fatalf("failed to create testFile: %s", err)
	}
	if _, err := testFile.WriteString("1\n2\n3\n4\n5\n6"); err != nil {
		t.Fatalf("failed to WriteString to testFile: %s", err)
	}
	testFile.Close()

	tw, err := NewTailWatcher()
	if err != nil {
		t.Fatalf("could not create TailWatcher: %s", err)
	}
	defer tw.Close()

	tail, err := tw.Add(fname, 5, nil, 5)
	if err != nil {
		t.Fatalf("failed to Add to TailWatcher: %s", err)
	}

	var s string
	for i := 0; i < 5; i++ {
		s += *tail.Next()
	}
	if s != "12345" {
		t.Fatalf("expect 12345 but got: %s\n", s)
	}

	tail2, err := tw.Lookup(fname + "wrong")
	if err == nil {
		t.Fatalf("should not find wrong path tail")
	}
	if tail2, err = tw.Lookup(fname); tail == nil { // or err != nil
		t.Fatalf("should find right path tail")
	}
	s = ""
	for i := 0; i < 5; i++ {
		s += *tail2.Next()
	}
	if s != "12345" {
		t.Fatalf("expect 12345 but got: %s\n", s)
	}
}
