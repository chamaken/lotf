// (C) 2014 by Ken-ichirou MATSUZAWA <chamas@h4.dion.ne.jp>
// Use of this source code is gdonened by a BSD-style
//
// このコードは Go package の bufio パッケージを参考にしました
//
// these code refers to bufio package in Go
//
// TailReader はシンプルな tail を実装するために作りました。末尾からファイルを読
// み込みます。例えば (ReadAt と Seek を実装する) ファイルの内容が
//
//     1<LF>
//     2:<LF>
//     3: 3<LF>
//     4: 44<LF>
//     5: 555<LF>
//
// の場合、最初の PrevBytes は最後の <LF> のみを返します。続く PrevBytes は
// ``<LF>5: 555'' です。更に ``<LF>4: 44'' と続き最後は ErrorStartOfFile エラー
// と共に ``1'' を返します。
//
// TailReader was created to implement simple *nix tail command. This read from
// end of file. For example, the content... (which implements ReadAt and Seeker)
//
//     1<LF>
//     2:<LF>
//     3: 3<LF>
//     4: 44<LF>
//     5: 555<LF>
//
// First PrevBytes() call returns the last <LF> only. Following PrevBytes()
// returns ``<LF>5: 555'' and ``<LF>4: 44'' and the last call returns
// ErrorStartOfFile and ``1''

package lotf

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	// logger "github.com/chamaken/logger"
)

type ReadAtSeeker interface {
	ReadAt(p []byte, off int64) (n int, err error)
	Seek(offset int64, whence int) (int64, error)
}

type TailReader struct {
	buf  []byte
	rd   io.ReaderAt
	tail int
	err  error
	pos  int64
	base int64
}

const (
	defaultBufSize           = 4096
	minReadBufferSize        = 16
	maxConsecutiveEmptyReads = 100
)

var ErrorStartOfFile = errors.New("lotf: no previous line")
var ErrorNegativeRead = errors.New("lotf: reader returned negative count from Read")
var ErrorEmpty = errors.New("lotf: empty")

func NewTailReaderSize(rd ReadAtSeeker, size int) (*TailReader, error) {
	if size < minReadBufferSize {
		size = minReadBufferSize
	}

	r := new(TailReader)
	return r, r.reset(make([]byte, size), rd)
}

// NewTailReader returns a new TailReader whose buffer has the default size.
func NewTailReader(rd ReadAtSeeker) (*TailReader, error) {
	return NewTailReaderSize(rd, defaultBufSize)
}

func (b *TailReader) String() string {
	return fmt.Sprintf("base: %d, pos: %d, tail: %d, err: %v, buf: %v", b.base, b.pos, b.tail, b.err, b.buf)
}

func (b *TailReader) reset(buf []byte, r ReadAtSeeker) (err error) {
	*b = TailReader{
		buf: buf,
		rd:  r,
	}
	pos, err := r.Seek(0, os.SEEK_CUR)
	if err != nil {
		return err
	}
	if b.pos, err = r.Seek(0, os.SEEK_END); err != nil {
		return err
	}
	if _, err = r.Seek(pos, os.SEEK_SET); err != nil {
		return err
	}

	if b.pos == 0 {
		return ErrorEmpty
	}
	b.base = b.pos

	return nil
}

func (b *TailReader) Tell() int64 {
	return b.pos + int64(b.tail)
}

func (b *TailReader) Rewind() {
	b.pos = b.base
	b.tail = 0
	b.err = nil
}

// fill reads a new chunk into the buffer.
func (b *TailReader) fill() {
	// Slide existing data to end.
	head := len(b.buf) - b.tail
	if head > 0 {
		b.pos -= int64(head)
		if b.pos <= 0 {
			if b.pos < 0 {
				head = len(b.buf) + int(b.pos)
				b.pos = 0
			}
			b.err = ErrorStartOfFile
		}
		b.tail += head
		copy(b.buf[head:], b.buf[:b.tail])
	}

	// Read new data: try a limited number of times.
	for i := maxConsecutiveEmptyReads; i > 0; i-- {
		n, err := b.rd.ReadAt(b.buf[:head], b.pos)
		if n < 0 {
			panic(ErrorNegativeRead)
		}
		if err != nil {
			b.err = err
			if err != io.EOF {
				return
			}
		}
		if n < head {
			copy(b.buf[head-n:head], b.buf[:n])
		}
		if n > 0 {
			return
		}
	}
	b.err = io.ErrNoProgress
}

func (b *TailReader) readErr() error {
	err := b.err
	if b.err != ErrorStartOfFile {
		b.err = nil
	}
	return err
}

func (b *TailReader) PrevBuffered() int { return b.tail }

func (b *TailReader) PrevSlice(delim byte) (line []byte, err error) {
	for {
		// Search buffer.
		if i := bytes.LastIndex(b.buf[:b.tail], []byte{delim}); i >= 0 {
			line = b.buf[i:b.tail]
			b.tail = i
			break
		}
		// Pending error?
		if b.err != nil {
			line = b.buf[:b.tail]
			b.tail = 0
			err = b.readErr()
			break
		}

		// Buffer full?
		if n := b.PrevBuffered(); n >= len(b.buf) {
			b.tail = 0
			line = b.buf
			err = bufio.ErrBufferFull
			break
		}

		b.fill() // buffer is not full
	}

	return
}

func (b *TailReader) PrevBytes(delim byte) (line []byte, err error) {
	// Use ReadSlice to look for array,
	// accumulating full buffers.
	var frag []byte
	var full [][]byte
	err = nil

	for {
		var e error
		frag, e = b.PrevSlice(delim)
		if e == nil { // got final fragment
			break
		}
		if e != bufio.ErrBufferFull { // unexpected error
			err = e
			break
		}

		// Make a copy of the buffer.
		buf := make([]byte, len(frag))
		copy(buf, frag)
		full = append(full, buf)
	}

	// Allocate new buffer to hold the full pieces and the fragment.
	n := 0
	var i int
	for i = range full {
		n += len(full[i])
	}
	n += len(frag)

	// Copy full pieces and fragment in.
	buf := make([]byte, n)
	for i := range full {
		n -= len(full[i])
		copy(buf[n:], full[i])
	}
	copy(buf, frag)
	return buf, err
}
