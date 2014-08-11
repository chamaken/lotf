package lotf

import (
	"bufio"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func TestPrevSlice(t *testing.T) {
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

	// prepare testfile
	if err = testFile.Truncate(0); err != nil {
		t.Fatalf("truncate testFile failed: %s", err)
	}
	if _, err = testFile.Seek(0, os.SEEK_SET); err != nil {
		t.Fatalf("seek testFile failed: %s", err)
	}
	if _, err = testFile.WriteString("1\n2:\n3: \n4: 4\n5: 55"); err != nil {
		t.Fatalf("write testFile failed: %s", err)
	}

	// each line
	tr, err := NewTailReader(testFile)
	if err != nil {
		t.Fatalf("failed to create TailReader: %s", err)
	}
	line, err := tr.PrevSlice('\n')
	if err != nil {
		t.Fatalf("failed to get PrevSlice: %s", err)
	}
	if string(line) != "\n5: 55" {
		t.Fatalf("expect line '\\n5: 55' but got: %s", line)
	}
	if line, err = tr.PrevSlice('\n'); err != nil {
		t.Fatalf("failed to get PrevSlice: %s", err)
	}
	if string(line) != "\n4: 4" {
		t.Fatalf("expect line '\\n4: 4' but got: %s", line)
	}
	if line, err = tr.PrevSlice('\n'); err != nil {
		t.Fatalf("failed to get PrevSlice: %s", err)
	}
	if string(line) != "\n3: " {
		t.Fatalf("expect line '\\n3: ' but got: %s", line)
	}
	if line, err = tr.PrevSlice('\n'); err != nil {
		t.Fatalf("failed to get PrevSlice: %s", err)
	}
	if string(line) != "\n2:" {
		t.Fatalf("expect line '\\n2:' but got: %s", line)
	}
	if line, err = tr.PrevSlice('\n'); err != ErrorStartOfFile {
		t.Fatalf("expect ErrorStartOfFile, but got: %s", err)
	}
	if string(line) != "1" {
		t.Fatalf("expect line '1' but got: %s", line)
	}

	// min buf size
	tr, err = NewTailReaderSize(testFile, 16)
	if err != nil {
		t.Fatalf("failed to create TailReader: %s", err)
	}
	line, err = tr.PrevSlice('\n')
	if err != nil {
		t.Fatalf("failed to get PrevSlice: %s", err)
	}
	if string(line) != "\n5: 55" {
		t.Fatalf("expect line '\\n5: 55' but got: %s", line)
	}

	// prepare new file content
	if err = testFile.Truncate(0); err != nil {
		t.Fatalf("truncate testFile failed: %s", err)
	}
	if _, err = testFile.Seek(0, os.SEEK_SET); err != nil {
		t.Fatalf("seek testFile failed: %s", err)
	}
	if _, err = testFile.WriteString("1\n2:\n3: \n4: 4\n5: 55\n6: 666\n7: 7777\n8: 88888\n9: 999999\na: aaaaaaa\nb: bbbbbbbb\nc: ccccccccc\nd: dddddddddd\ne: eeeeeeeeeee\nf: ffffffffffff\n10: 000000000000\n"); err != nil {
		t.Fatalf("write testFile failed: %s", err)
	}
	if tr, err = NewTailReader(testFile); err != nil {
		t.Fatalf("failed to create TailReader: %s", err)
	}
	if line, err = tr.PrevSlice('\n'); err != nil {
		t.Fatalf("failed to get PrevSlice: %s", err)
	}
	if string(line) != "\n" {
		t.Fatalf("expect \\n but got: %s", line)
	}
	if line, err = tr.PrevSlice('\n'); err != nil {
		t.Fatalf("failed to get PrevSlice: %s", err)
	}
	if string(line) != "\n10: 000000000000" {
		t.Fatalf("expect line '\\n10: 000000000000' but got: %s", line)
	}

	// min buf size
	if tr, err = NewTailReaderSize(testFile, 16); err != nil {
		t.Fatalf("failed to create TailReader: %s", err)
	}
	if line, err = tr.PrevSlice('\n'); err != nil {
		t.Fatalf("failed to get PrevSlice: %s", err)
	}
	if string(line) != "\n" {
		t.Fatalf("expect \\n but got: %s", line)
	}
	// next call of PrevSlice should be err
}

func TestTell(t *testing.T) {
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

	// prepare file content
	if err = testFile.Truncate(0); err != nil {
		t.Fatalf("truncate testFile failed: %s", err)
	}
	if _, err = testFile.Seek(0, os.SEEK_SET); err != nil {
		t.Fatalf("seek testFile failed: %s", err)
	}
	s := "1\n2:\n3: \n4: 4\n5: 55\n6: 666\n7: 7777\n8: 88888\n9: 999999\na: aaaaaaa\nb: bbbbbbbb\nc: ccccccccc\nd: dddddddddd\ne: eeeeeeeeeee\nf: ffffffffffff\n10: 000000000000\n"
	if _, err = testFile.WriteString(s); err != nil {
		t.Fatalf("write testFile failed: %s", err)
	}

	tr, err := NewTailReader(testFile)
	if err != nil {
		t.Fatalf("failed to create TailReader: %s", err)
	}
	if int(tr.Tell()) != len(s) {
		t.Fatalf("expect %d, but got: %d", len(s), tr.Tell())
	}

	line, err := tr.PrevSlice('\n'); 
	if err != nil {
		t.Fatalf("failed to get PrevSlice: %s", err)
	}
	if string(line) != "\n" {
		t.Fatalf("expect \\n but got: %s", line)
	}
	if int(tr.Tell()) != len(s) - 1 {
		t.Fatalf("expect %d, but got: %d", len(s) - 1, tr.Tell())
	}
	if line, err = tr.PrevSlice('\n'); err != nil {
		t.Fatalf("failed to get PrevSlice: %s", err)
	}
	if string(line) != "\n10: 000000000000" {
		t.Fatalf("expect line '\\n10: 000000000000' but got: %s", line)
	}
	if int(tr.Tell()) != len(s) - 2 - 0x10 {
		t.Fatalf("expect %d, but got: %d", len(s) - 2 - 0x10, tr.Tell())
	}

	// move file pointer to the beginnig
	for err == nil {
		_, err = tr.PrevSlice('\n')
	}
	if err != ErrorStartOfFile {
		t.Fatalf("expect ErrorStartOfFile, but got: %s", err)
	}
	if tr.Tell() != 0 {
		t.Fatalf("Tell() of StartOfFile should return 0, but got: %d", tr.Tell())
	}

	tr.Rewind()
	if int(tr.Tell()) != len(s) {
		t.Fatalf("Tell() of EndOfFile should return %d, but got: %d", len(s), tr.Tell())
	}
}

func TestPrevBytes(t *testing.T) {
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

	// prepare file content
	if err = testFile.Truncate(0); err != nil {
		t.Fatalf("truncate testFile failed: %s", err)
	}
	if _, err = testFile.Seek(0, os.SEEK_SET); err != nil {
		t.Fatalf("seek testFile failed: %s", err)
	}
	s := "11: 1111111111111\n1\n2:\n3: \n4: 4\n5: 55\n6: 666\n7: 7777\n8: 88888\n9: 999999\na: aaaaaaa\nb: bbbbbbbb\nc: ccccccccc\nd: dddddddddd\ne: eeeeeeeeeee\nf: ffffffffffff\n10: 000000000000\n"
	if _, err = testFile.WriteString(s); err != nil {
		t.Fatalf("write testFile failed: %s", err)
	}

	// min size buffer
	tr, err := NewTailReaderSize(testFile, 16)
	if err != nil {
		t.Fatalf("failed to create TailReader: %s", err)
	}
	line, err := tr.PrevSlice('\n') // returns \n only
	if line, err = tr.PrevSlice('\n'); err != bufio.ErrBufferFull {
		t.Fatalf("expect bufio.ErrBufferFull, but got: %v", err)
	}
	if string(line) != "10: 000000000000" {
		t.Fatalf("expect 0 * 16, but got: %v", line)
	}
	if line, err = tr.PrevBytes('\n'); err != nil {
		t.Fatalf("failed to PrevBytes: %s", err)
	}
	if string(line) != "\n" {
		t.Fatalf("expect string \\n, but got: %s", string(line))
	}		

	tr.Rewind()
	line, err = tr.PrevBytes('\n') // returns \n only
	if line, err = tr.PrevBytes('\n'); err != nil {
		t.Fatalf("failed to PrevBytes: %s", err)
	}
	if string(line) != "\n10: 000000000000" {
		t.Fatalf("expect \\n 0 * 16, but got: %v", line)
	}

	// progress to the beginning
	for err == nil {
		line, err = tr.PrevBytes('\n')
	}
	if err != ErrorStartOfFile {
		t.Fatalf("expect ErrorStartOfFile, but got: %s", err)
	}
	if string(line) != "11: 1111111111111" {
		t.Fatalf("expect '11: 1111111111111', but got: %s", string(line))
	}
}

func TestSmallFile(t *testing.T) {
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

	// prepare file content
	if err = testFile.Truncate(0); err != nil {
		t.Fatalf("truncate testFile failed: %s", err)
	}
	if _, err = testFile.Seek(0, os.SEEK_SET); err != nil {
		t.Fatalf("seek testFile failed: %s", err)
	}

	// empty file
	if _, err = NewTailReader(testFile); err != ErrorEmpty {
		t.Fatalf("should return ErrorEmpty but got: %v", err)
	}

	// one line without LF
	if _, err = testFile.WriteString("test"); err != nil {
		t.Fatalf("failed to WriteString: %s", err)
	}
	tr, err := NewTailReader(testFile)
	if err != nil {
		t.Fatalf("failed to create TailReader: %s", err)
	}
	line, err := tr.PrevSlice('\n')
	if string(line) != "test" {
		t.Fatalf("expect string 'test', but got: %s", string(line))
	}
	if err != ErrorStartOfFile {
		t.Fatalf("expect ErrorStartOfFile but got: %s", err)
	}

	// put delim just buf border
	if err = testFile.Truncate(0); err != nil {
		t.Fatalf("truncate testFile failed: %s", err)
	}
	if _, err = testFile.Seek(0, os.SEEK_SET); err != nil {
		t.Fatalf("seek testFile failed: %s", err)
	}
	if _, err = testFile.WriteString("0123456789abcdef\n0123456789abcde"); err != nil {
		t.Fatalf("failed to WriteString: %s", err)
	}
	if tr, err = NewTailReaderSize(testFile, 16); err != nil {
		t.Fatalf("failed to create TailReader: %s", err)
	}
	// first 16byte
	if line, err = tr.PrevSlice('\n'); err != nil {
		t.Fatalf("failed to PrevSlice: %s", err)
	}
	if string(line) != "\n0123456789abcde" {
		t.Fatalf("expect string '\\n0123456789abcde' but got: %s", string(line))
	}
	if tr.Tell() != 16 {
		t.Fatalf("Tell() should return 16, but got: %d", tr.Tell())
	}
	// 2nd 16byte
	if line, err = tr.PrevSlice('\n'); err != ErrorStartOfFile {
		t.Fatalf("failed to PrevSlice: %s", err)
	}
	if string(line) != "0123456789abcdef" {
		t.Fatalf("expect string '0123456789abcdef' but got: %s", string(line))
	}
	if tr.Tell() != 0 {
		t.Fatalf("Tell() should return 16, but got: %d", tr.Tell())
	}
}
