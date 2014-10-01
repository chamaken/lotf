package lotf

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"
)

const TMP_PREFIX = "_lotf_tmp"

func init() {
	dir, _ := ioutil.TempDir("", TMP_PREFIX)
	parent := filepath.Dir(dir)
	globs, _ := filepath.Glob(filepath.Join(parent, TMP_PREFIX+"*"))
	for _, d := range globs {
		os.RemoveAll(d)
	}
}

func TestFileLinesFilter(t *testing.T) {
	// prepare
	dir, err := ioutil.TempDir("", TMP_PREFIX)
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

	// create filter
	filter, err := RegexpFilter("!testfilter")
	if err != nil {
		t.Fatalf("failed to create filter: %s", err)
	}
	t.Logf("testfile: %s", testFile.Name())
	tail, err := tw.Add(testFile.Name(), 3, filter, 3)
	if err != nil {
		t.Fatalf("failed to Add to TailWatcher: %s", err)
	}

	//  0 1 23 45 6 78 9
	// "\n\na\nb\n\nc\nd"
	s := *tail.WaitNext()
	if s != "a" {
		t.Fatalf("expect `a', but got: %s", s)
	}
	s = *tail.WaitNext()
	if s != "b" {
		t.Fatalf("expect `b', but got: %s", s)
	}
	s = *tail.WaitNext()
	if s != "c" {
		t.Fatalf("expect `c', but got: %s", s)
	}
}

func TestTailWatcher(t *testing.T) {
	// prepare
	dir, err := ioutil.TempDir("", TMP_PREFIX)
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
	dir, err := ioutil.TempDir("", TMP_PREFIX)
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
	if err != nil {
		t.Fatalf("failed to Add to TailWatcher: %s", err)
	}

	tailch := make(chan *string)
	done := make(chan bool)
	var line *string

	go func() {
		for {
			p := tail.WaitNext()
			if p == nil {
				break
			}
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
	dir, err := ioutil.TempDir("", TMP_PREFIX)
	if err != nil {
		t.Fatalf("TempDir failed: %s", err)
	}
	t.Logf("tmpdir: %s", dir)
	defer os.RemoveAll(dir)
	fname := filepath.Join(dir, "TailWatcher.testfile")
	tmpfname := filepath.Join(dir, "TailWatcher.tmpfile")

	testFile1, err := os.OpenFile(fname, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		t.Fatalf("failed to create testFile: %s", err)
	}
	defer testFile1.Close()
	// file contains 6 lines - a b c d e f
	if _, err = testFile1.WriteString("ABCDEFGHIJKLMNOPQRSTUVWXYZa\nb\nc\nd\ne\nf\n"); err != nil {
		t.Fatalf("write testFile failed: %s", err)
	}

	tw, err := NewTailWatcher()
	if err != nil {
		t.Fatalf("could not create TailWatcher: %s", err)
	}
	defer tw.Close()
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
		s += *tail.WaitNext()
	}
	if s != "bcdef" {
		t.Fatalf("expect bcdef but got: %s\n", s)
	}

	// rename
	if err := os.Rename(fname, tmpfname); err != nil {
		t.Fatalf("failed to rename testfile: %s", err)
	}
	// and recreate with lines - 1 2 3 4 5 6
	testFile2, err := os.OpenFile(fname, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		t.Fatalf("failed to create testFile: %s", err)
	}
	defer testFile2.Close()
	if _, err = testFile2.WriteString("1\n2\n3\n4\n5\n6\n"); err != nil {
		t.Fatalf("write testFile failed: %s", err)
	}

	for i := 0; i < 6; i++ {
		s += *tail.WaitNext()
	}
	if s != "bcdef123456" {
		t.Fatalf("expect bcdef123456 but got: %s\n", s)
	}

	if _, err = testFile2.WriteString("7\n"); err != nil {
		t.Fatalf("write testFile failed: %s", err)
	}
	if *tail.WaitNext() != "7" {
		t.Fatalf("expect 7 but got: %s\n", s)
	}
}

func TestFilesInSameDir(t *testing.T) {
	// prepare
	dir, err := ioutil.TempDir("", TMP_PREFIX)
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

	tw, err := NewTailWatcher()
	if err != nil {
		t.Fatalf("could not create TailWatcher: %s", err)
	}
	defer tw.Close()
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

	for i := 0; i < 10; i++ {
		s := fmt.Sprintf("TEST%d\ntest%d", i, i)
		if _, err = testFiles[i].WriteString(s); err != nil {
			t.Fatalf("write testFile failed: %s", err)
		}
	}

	for i := 0; i < 10; i++ {
		p := tails[i].WaitNext()
		if p == nil {
			t.Fatal("expect valid pointer, but got nil")
		}
		s := fmt.Sprintf("TEST%d", i)
		if *p != s {
			t.Fatalf("expect string %s, but got: %s", s, *p)
		}
	}

	// delete file and read the rest
	for i := 0; i < 10; i++ {
		if err = testFiles[i].Close(); err != nil {
			t.Fatalf("failed to close testFile: %s", err)
		}
		if err = os.Remove(testFiles[i].Name()); err != nil {
			t.Fatalf("failed to remove testFile: %s", err)
		}
		p := tails[i].WaitNext()
		if p == nil {
			t.Fatal("expect valid pointer, but got nil")
		}
		s := fmt.Sprintf("test%d", i)
		if *p != s {
			t.Fatalf("expect string %s, but got: %s", s, *p)
		}
	}
}

func TestLookup(t *testing.T) {
	// prepare
	dir, err := ioutil.TempDir("", TMP_PREFIX)
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
	go func() {
		for err := range tw.Error {
			t.Fatalf("error received: %s", err)
		}
	}()

	tail, err := tw.Add(fname, 5, nil, 5)
	if err != nil {
		t.Fatalf("failed to Add to TailWatcher: %s", err)
	}

	var s string
	for i := 0; i < 5; i++ {
		s += *tail.WaitNext()
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
		s += *tail2.WaitNext()
	}
	if s != "12345" {
		t.Fatalf("expect 12345 but got: %s\n", s)
	}
}

func TestLastZero(t *testing.T) {
	dir, err := ioutil.TempDir("", TMP_PREFIX)
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
	go func() {
		for err := range tw.Error {
			t.Fatalf("error received: %s", err)
		}
	}()

	tail, err := tw.Add(fname, 1, nil, 0)
	if err != nil {
		t.Fatalf("failed to Add to TailWatcher: %s", err)
	}

	s := tail.Next()
	if s != nil {
		t.Fatalf("expect nil but got: %s", *s)
	}
}

func TestParentDisappear(t *testing.T) {
	dir, err := ioutil.TempDir("", TMP_PREFIX)
	if err != nil {
		t.Fatalf("TempDir failed: %s", err)
	}
	defer os.RemoveAll(dir)

	// create 10 removing parent dirs in the tmpdir
	// and two files in a removing dir
	var testDirs [10]string
	var testFiles [20]*os.File
	for i := 0; i < 10; i++ {
		testDirs[i], err = ioutil.TempDir(dir, "rmtest")
		if err != nil {
			t.Fatalf("failed to create testDir: %s", err)
		}
		if testFiles[i], err = os.OpenFile(filepath.Join(testDirs[i], "TailWatcher1"),
			os.O_RDWR|os.O_CREATE, 0666); err != nil {
			t.Fatalf("failed to create testFile: %s", err)
		}
		if err := testFiles[i].Close(); err != nil {
			t.Fatalf("failed to close testFile: %s", err)
		}
		if testFiles[i+10], err = os.OpenFile(filepath.Join(testDirs[i], "TailWatcher2"),
			os.O_RDWR|os.O_CREATE, 0666); err != nil {
			t.Fatalf("failed to create testFile: %s", err)
		}
		if err := testFiles[i+10].Close(); err != nil {
			t.Fatalf("failed to close testFile: %s", err)
		}
	}

	tw, err := NewTailWatcher()
	if err != nil {
		t.Fatalf("could not create TailWatcher: %s", err)
	}
	defer func() {
		t.Logf("-------- call tw.Close()")
		// tw.Close()
		t.Logf("-------- returned tw.Close()")
	}()
	go func() {
		for err := range tw.Error {
			t.Fatalf("error received: %s", err)
		}
	}()

	for i := 0; i < 20; i++ {
		if _, err = tw.Add(testFiles[i].Name(), 1, nil, 0); err != nil {
			t.Fatalf("failed to Add to TailWatcher: %s", err)
		}
	}
	// checking
	if len(tw.tails) != 30 { // 20 files 10 parent dirs
		t.Fatalf("len(tails) should be 20, but got: %d", len(tw.tails))
	}
	if len(tw.dirs) != 10 {
		t.Fatalf("len(dirs) should be 10, but got: %d", len(tw.dirs))
	}

	// remove a few
	for i := 0; i < 3; i++ {
		if err = os.RemoveAll(testDirs[i]); err != nil {
			t.Fatalf("failed to RemoveAll: %s", err)
		}
	}
	// wait receiving events above.
	time.Sleep(100 * time.Millisecond)

	// check again
	if len(tw.tails) != 21 { // 7 dirs remainded
		t.Fatalf("len(tails) should be 14, but got: %d", len(tw.tails))
	}
	if len(tw.dirs) != 7 {
		t.Fatalf("len(dirs) should be 7, but got: %d", len(tw.dirs))
	}
}
