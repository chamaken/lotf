package lotf

import (
	"bufio"
	"bytes"
	"fmt"
	inotify "github.com/chamaken/inotify"
	"github.com/golang/glog"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
)

const (
	BUFSIZ       = 8192
	INOTIFY_MASK = inotify.IN_DELETE_SELF | inotify.IN_MOVE_SELF | inotify.IN_CREATE | inotify.IN_MOVE | inotify.IN_DELETE | inotify.IN_MODIFY
)

// tail -- output the last part of file(s)
// Copyright (C) 1989-2014 Free Software Foundation, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

// FileLines sets the offset to nLines lines from the last. This does not means
// EOF if the file is ended with no newline.
func FileLines(file *os.File, nLines int) (int64, error) {
	if nLines < 0 {
		return -1, syscall.EINVAL
	}

	fi, err := file.Stat()
	if err != nil {
		if glog.V(1) {
			glog.Infof("File.Stat(): %s", err)
		}
		return -1, err
	}
	if fi.Sys().(*syscall.Stat_t).Mode&syscall.S_IFMT != syscall.S_IFREG {
		return -1, fmt.Errorf("support regular file only")
	}

	pos := fi.Size()
	if pos == 0 {
		return 0, nil
	}
	buffer := make([]byte, BUFSIZ)
	var bytesRead int

	// Set 'bytesRead' to the size of the last, probably partial, buffer;
	// 0 < 'bytesRead' <= 'BUFSIZ'
	bytesRead = int(pos % BUFSIZ)
	if bytesRead == 0 {
		bytesRead = BUFSIZ
	}

	// Make 'pos' a multiple of 'BUFSIZ' (0 if the file is short), so that all
	// reads will be on block (BUFSIZ) boundaries, which might increase efficiency.
	pos -= int64(bytesRead)
	if _, err := file.Seek(pos, os.SEEK_SET); err != nil {
		if glog.V(1) {
			glog.Infof("File.Seek(%d, SEEK_SET): %s", pos, err)
		}
		return -1, err
	}
	if bytesRead, err = file.Read(buffer[:bytesRead]); err != nil {
		if glog.V(1) {
			glog.Infof("File.Read(): %s", err)
		}
		return -1, err
	}

	// Not decrement incomplete line

	var nlPos int
LOOP:
	for {
		nlPos = bytesRead
		for nlPos != 0 { // in case of buffer[0] == '\n'
			prevNL := nlPos
			nlPos = bytes.LastIndex(buffer[:prevNL], []byte("\n"))
			if nlPos == -1 {
				break
			}
			if nLines == 0 {
				break LOOP
			}
			nLines--
		}
		if pos == 0 {
			// Just start or not enough lines in the file
			return file.Seek(0, os.SEEK_SET)
		}
		pos -= BUFSIZ
		if _, err = file.Seek(pos, os.SEEK_SET); err != nil {
			if glog.V(1) {
				glog.Infof("File.Seek(%d, SEEK_SET): %s", pos, err)
			}
			return -1, err
		}

		if bytesRead, err = file.Read(buffer); err != nil {
			if glog.V(1) {
				glog.Infof("File.Read(): %s", err)
			}
			return -1, err
		}
	}

	return file.Seek(pos+int64(nlPos+1), os.SEEK_SET)
}

type TailName struct {
	name    string   // file absname
	file    *os.File // watching file
	lastp   int64    // file position last newline after 1
	lines   *Blockq  // stores lines with no NL
	filter  Filter   // lines is not store if this returns false
	current *Element
}

type Tail interface {
	Name() string
	WaitNext() *string
	Next() *string
	Reset()
	Clone() Tail
	SetFilter(Filter)
}

// subroutine of event handlers. This function reads lines from tail.lastp
// and stores it tail.Lines. line which is not ended with newline will not
// store and not increment tail.lastp
func (tail *TailName) readlines(errch chan<- error) {
	var line []byte
	var err error

	if _, err = tail.file.Seek(tail.lastp, os.SEEK_SET); err != nil {
		glog.Infof("File.Seek(%d, SEEK_SET): %s", tail.lastp, err)
		errch <- err
		return
	}
	r := bufio.NewReader(tail.file)
	for {
		line, err = r.ReadBytes(byte('\n'))
		if err == io.EOF {
			return
		} else if err != nil {
			glog.Infof("File.ReadBytes(): %s", err)
			errch <- err
			return
		}
		if tail.filter == nil || tail.filter.Filter(string(line[:len(line)-1])) {
			tail.lines.Add(string(line[:len(line)-1]))
		}
		tail.lastp += int64(len(line))
	}
}

// IN_CREATE event handler. This function opens file named tail.name and reads lines.
// tail.file should be nil if this function is called.
func (tail *TailName) handleCreate(errch chan<- error) {
	var err error

	if tail.file != nil {
		errch <- fmt.Errorf("open already opened file")
		if err := tail.file.Close(); err != nil {
			glog.Infof("File.Close(): %s", err)
			errch <- err
		}
	}

	tail.file, err = os.Open(tail.name)
	if err != nil {
		glog.Infof("File.Open(%s): %s", tail.name, err)
		errch <- err
		return
	}
	tail.lastp = 0
	tail.readlines(errch)
}

// IN_DELETE or IN_MOVED event handler. tail.tailp will differ is last modification
// was not ended with newline so that the last line will store only in the case.
// This function close tail.file and invalidate it after that.
func (tail *TailName) handleDisappear(errch chan<- error) {
	fi, err := tail.file.Stat()
	if err != nil {
		glog.Infof("File.Stat(): %s", err)
		errch <- err
		return
	}
	// read unfinished one line
	for fi.Size() > tail.lastp {
		if _, err = tail.file.Seek(tail.lastp, os.SEEK_SET); err != nil {
			glog.Infof("File.Seek(%d, SEEK_SET): %s", tail.lastp, err)
			errch <- err
		}
		r := bufio.NewReader(tail.file)
		line, err := r.ReadBytes(byte('\n'))
		// add line even if it does not end with LF
		if err != nil && err != io.EOF {
			glog.Infof("File.ReadBytes(): %s", err)
			errch <- err
		}
		if tail.filter == nil || tail.filter.Filter(string(line[:len(line)-1])) {
			if line[len(line)-1] == byte('\n') {
				tail.lines.Add(string(line[:len(line)-1]))
			} else {
				tail.lines.Add(string(line))
			}
		}
		tail.lastp += int64(len(line))
	}

	// close and invalidate TailName.file
	if err = tail.file.Close(); err != nil {
		glog.Infof("File.Close(): %s", err)
		errch <- err
	}
	tail.file = nil
}

// IN_MODIFY event handler. This function checks file size and store lines if the file
// was grown up.
func (tail *TailName) handleModify(errch chan<- error) {
	fi, err := tail.file.Stat()
	if err != nil {
		glog.Infof("File.Stat(): %s", err)
		errch <- err
		return
	}
	if fi.Size() > tail.lastp {
		tail.readlines(errch)
	}
	// XXX: else if fi.Size() < tail.lastp { reread from first? }
}

func (tail *TailName) Name() string {
	return tail.name
}

func (tail *TailName) WaitNext() *string {
	next := tail.current.WaitNext()
	if next == nil { // TailWatcher has closed
		// XXX: what should do after Remove()
		return nil
	}
	tail.current = next
	s := tail.current.Value.(string)
	return &s
}

func (tail *TailName) Next() *string {
	e := tail.current.Next()
	if e == nil {
		return nil
	}
	tail.current = e
	s := e.Value.(string)
	return &s
}

func (tail *TailName) Reset() {
	tail.current = tail.lines.head
}

func (tail *TailName) Clone() Tail {
	return &TailName{
		name:    tail.name,
		file:    tail.file,
		lastp:   tail.lastp,
		lines:   tail.lines,
		filter:  tail.filter,
		current: tail.lines.head,
	}
}

func (tail *TailName) SetFilter(filter Filter) {
	tail.filter = filter
}

func (tail *TailName) String() string {
	if tail.filter != nil {
		return fmt.Sprintf("%s | %s", tail.name, tail.filter)
	}
	return tail.name
}

type TailWatcher struct {
	watch  *inotify.Watcher
	tails  map[string]*TailName // key: abs pathname or parent dirname if TailName is nil
	dirs   map[string]int       // key: dirname, value: refcount
	mu     sync.Mutex           // to sync tails map
	Error  <-chan error
	closed bool
}

// TailWatcher constructor
func NewTailWatcher() (*TailWatcher, error) {
	watcher, err := inotify.NewWatcher()
	if err != nil {
		if glog.V(1) {
			glog.Infof("inotify.NewWatcher(): %s", err)
		}
		return nil, err
	}

	tw := &TailWatcher{
		watcher,
		make(map[string]*TailName),
		make(map[string]int),
		*new(sync.Mutex),
		watcher.Error,
		false,
	}
	go tw.follow()
	return tw, nil
}

// Watcher event dispatcher
func (tw *TailWatcher) follow() {
	for ev := range tw.watch.Event {
		// need Lock?
		tail, found := tw.tails[ev.Name]
		if !found {
			continue
		}
		switch {
		case ev.Mask&inotify.IN_CREATE != 0:
			tail.handleCreate(tw.watch.Error)
		case ev.Mask&(inotify.IN_DELETE|inotify.IN_MOVE) != 0:
			tail.handleDisappear(tw.watch.Error)
		case ev.Mask&inotify.IN_MODIFY != 0:
			tail.handleModify(tw.watch.Error)
		case ev.Mask&(inotify.IN_DELETE_SELF|inotify.IN_MOVE_SELF) != 0:
			tw.handleParentDisappear(ev.Name, tw.watch.Error)
		}
	}
}

func (tw *TailWatcher) Close() error {
	if tw.closed {
		return os.NewSyscallError("closed", syscall.EBADF)
	}
	if err := tw.watch.Close(); err != nil {
		return err
	}

	tw.mu.Lock()
	defer tw.mu.Unlock()
	for _, tail := range tw.tails {
		if tail == nil { // parent directory
			continue
		}
		tail.lines.Done()
		if tail.file == nil {
			continue
		}
		if err := tail.file.Close(); err != nil {
			if glog.V(1) {
				glog.Infof("File.Close(): %s", err)
			}
			return err
		}
	}
	tw.tails = nil
	tw.dirs = nil
	tw.closed = true

	return nil
}

func (tw *TailWatcher) Add(pathname string, maxline int, filter Filter, lines int) (Tail, error) {
	if tw.closed {
		return nil, os.NewSyscallError("closed", syscall.EBADF)
	}

	var tail *TailName
	var absname string // TailName.name
	var dirname string // watch dir name
	var file *os.File  // TailName.file
	var pos int64      // TailName.lastp
	var q *Blockq      // TailName.Lines

	var tr *TailReader
	var line, lastLine []byte
	var err error

	// normalize pathname
	if absname, err = filepath.Abs(pathname); err != nil {
		if glog.V(1) {
			glog.Infof("filepath.Abs(): %s", err)
		}
		return nil, err
	}
	if _, found := tw.tails[absname]; found {
		return nil, fmt.Errorf("already watching: %s", absname)
	}
	dirname = filepath.Dir(absname)

	// open file
	if file, err = os.Open(absname); err != nil {
		if glog.V(1) {
			glog.Infof("Open(%s): %s", absname, err)
		}
		return nil, err
	}

	// create list for last lines
	if q, err = NewBlockq(maxline); err != nil {
		if glog.V(1) {
			glog.Infof("NewBlockq(): %s", err)
		}
		goto ERR_CLOSE
	}

	// create TailReader and adjust to last NL
	tr, err = NewTailReader(file)
	if err == ErrorEmpty {
		lines = 0
	} else if err != nil {
		goto ERR_CLOSE
	} else {
		pos = tr.Tell()
		lastLine, err = tr.PrevBytes('\n')
		if err == ErrorStartOfFile {
			lines = 0
		} else if err != nil {
			goto ERR_CLOSE
		} else if len(lastLine) != 1 { // not ended with '\n'
			pos -= int64(len(lastLine) - 1)
		}
	}

	// stores last lines from TailReader
	for lines > 0 {
		line, err = tr.PrevBytes('\n')
		if err != nil {
			if err != ErrorStartOfFile {
				if glog.V(1) {
					glog.Infof("TailReader.PrevBytes(): %s", err)
				}
				goto ERR_CLOSE
			}
			lines = 0
		}
		if len(line) > 0 && line[0] == '\n' {
			line = line[1:]
		}
		if filter == nil || filter.Filter(string(line)) {
			q.AddHead(string(line))
			lines--
		}
	}

	if _, err = file.Seek(pos, os.SEEK_SET); err != nil {
		if glog.V(1) {
			glog.Infof("File.Seek(%d, SEEK_SET): %s", pos, err)
		}
		goto ERR_CLOSE
	}

	tail = &TailName{
		name:    absname,
		file:    file,
		lastp:   pos,
		lines:   q,
		filter:  filter,
		current: q.head,
	}

	tw.mu.Lock()
	defer tw.mu.Unlock()

	// check again with holding lock
	if _, found := tw.tails[absname]; found {
		err = fmt.Errorf("already watching: %s", absname)
		goto ERR_CLOSE
	}
	if refcnt, found := tw.dirs[dirname]; !found {
		err = tw.watch.AddWatchFilter(dirname, INOTIFY_MASK,
			func(e *inotify.Event) bool {
				_, found := tw.tails[e.Name]
				return found
			})
		if err != nil {
			if glog.V(1) {
				glog.Infof("AddWatchFilter(): %s", err)
			}
			goto ERR_CLOSE
		}
		tw.dirs[dirname] = 1
		tw.tails[dirname] = nil
	} else {
		tw.dirs[dirname] = refcnt + 1
	}
	tw.tails[absname] = tail

	return tail, nil

ERR_CLOSE:
	file.Close()
	return nil, err
}

func (tw *TailWatcher) Lookup(pathname string) (Tail, error) {
	if tw.closed {
		return nil, os.NewSyscallError("closed", syscall.EBADF)
	}

	// normalize pathname
	absname, err := filepath.Abs(pathname)
	if err != nil {
		if glog.V(1) {
			glog.Info("filepath.Abs(): %s", err)
		}
		return nil, err
	}

	tw.mu.Lock()
	defer tw.mu.Unlock()
	// tail == nil means parent directory
	if tail, found := tw.tails[absname]; tail != nil && found {
		return tail.Clone(), nil
	}
	return nil, fmt.Errorf("no such a watcher: %s", absname)
}

func (tw *TailWatcher) Remove(pathname string) error {
	if tw.closed {
		return os.NewSyscallError("closed", syscall.EBADF)
	}

	// normalize pathname
	absname, err := filepath.Abs(pathname)
	if err != nil {
		if glog.V(1) {
			glog.Infof("filepath.Abs(): %s", err)
		}
		return err
	}
	dirname := filepath.Dir(absname)

	tw.mu.Lock()
	defer tw.mu.Unlock()
	tail, found := tw.tails[absname]
	if !found || tail == nil {
		return fmt.Errorf("no such a watcher: %s", absname)
	}
	refcnt, found := tw.dirs[dirname]
	if !found {
		// FATAL
		return fmt.Errorf("no such a dir: %s", dirname)
	}

	if tail.file != nil {
		if err := tail.file.Close(); err != nil {
			if glog.V(1) {
				glog.Infof("File.Close(): %s", err)
			}
			return err
		}
	}
	tail.lines.Done()
	delete(tw.tails, absname)

	if refcnt == 1 { // the last one
		if err := tw.watch.RemoveWatch(dirname); err != nil {
			if glog.V(1) {
				glog.Infof("inotify.RemoveWatch(): %s", err)
			}
			return err
		}
		delete(tw.tails, dirname)
		delete(tw.dirs, dirname)
		return nil
	} else {
		tw.dirs[dirname] = refcnt - 1
	}
	return nil
}

func (tw *TailWatcher) handleParentDisappear(dname string, errch chan<- error) {
	glog.Errorf("parent directory disappeared: %s", dname)
	tw.mu.Lock()
	defer tw.mu.Unlock()

	for name, tail := range tw.tails {
		if !strings.HasPrefix(name, dname) {
			continue
		}
		delete(tw.tails, name)
		if tail == nil {
			continue
		}
		tail.lines.Done()
		if tail.file != nil {
			if err := tail.file.Close(); err != nil {
				if glog.V(1) {
					glog.Infof("File.Close(): %s", err)
				}
				errch <- err
			}
		}
	}
	delete(tw.dirs, dname)
}
