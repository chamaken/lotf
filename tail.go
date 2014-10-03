package lotf

import (
	"bufio"
	"fmt"
	"github.com/chamaken/fsnotify" // use inotify branch
	"github.com/golang/glog"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
)

const (
	BUFSIZ = 8192
)

type TailName struct {
	name    string  // file absname
	lastp   int64   // file position last newline after 1
	lines   *Blockq // stores lines with no NL
	filter  Filter  // lines is not store if this returns false
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

	tail_file, err := os.Open(tail.name)
	if err != nil {
		errch <- err
		return
	}
	defer func() {
		if err := tail_file.Close(); err != nil {
			errch <- err
		}
	}()
	if _, err = tail_file.Seek(tail.lastp, os.SEEK_SET); err != nil {
		glog.Infof("File.Seek(%d, SEEK_SET): %s", tail.lastp, err)
		errch <- err
		return
	}
	r := bufio.NewReader(tail_file)
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

	if tail.lastp != -1 {
		errch <- fmt.Errorf("open already opened file")
	}

	tail_file, err := os.Open(tail.name)
	if err != nil {
		glog.Infof("File.Open(%s): %s", tail.name, err)
		errch <- err
		return
	}
	defer func() {
		if err := tail_file.Close(); err != nil {
			errch <- err
		}
	}()

	tail.lastp = 0
	tail.readlines(errch)
}

// IN_DELETE or IN_MOVED event handler. tail.tailp will differ is last modification
// was not ended with newline so that the last line will store only in the case.
// This function close tail.file and invalidate it after that.
func (tail *TailName) handleDisappear(errch chan<- error) {
	tail.lastp = -1
}

// IN_MODIFY event handler. This function checks file size and store lines if the file
// was grown up.
func (tail *TailName) handleModify(errch chan<- error) {
	tail_file, err := os.Open(tail.name)
	if err != nil {
		errch <- err
		return
	}
	defer func() {
		if err := tail_file.Close(); err != nil {
			errch <- err
		}
	}()
	fi, err := tail_file.Stat()
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
	watch *fsnotify.Watcher
	tails map[string]*TailName // key: abs pathname or parent dirname if TailName is nil
	dirs  map[string]int       // key: dirname, value: refcount
	mu    sync.Mutex           // to sync tails map
	Error <-chan error
}

// TailWatcher constructor
func NewTailWatcher() (*TailWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		if glog.V(1) {
			glog.Infof("fsnotify.NewWatcher(): %s", err)
		}
		return nil, err
	}

	tw := &TailWatcher{
		watcher,
		make(map[string]*TailName),
		make(map[string]int),
		*new(sync.Mutex),
		watcher.Errors,
	}
	go tw.follow()
	return tw, nil
}

// Watcher event dispatcher
func (tw *TailWatcher) follow() {
	for ev := range tw.watch.Events {
		// need Lock?
		tail, found := tw.tails[ev.Name]
		if !found {
			continue
		}
		switch {
		case ev.Op&fsnotify.Create != 0:
			tail.handleCreate(tw.watch.Errors)
		case ev.Op&(fsnotify.Remove|fsnotify.Rename) != 0:
			if tail == nil { // parent directory
				tw.handleParentDisappear(ev.Name, tw.watch.Errors)
			} else {
				tail.handleDisappear(tw.watch.Errors)
			}
		case ev.Op&fsnotify.Write != 0:
			tail.handleModify(tw.watch.Errors)
		}
	}
}

func (tw *TailWatcher) Close() error {
	if tw.watch == nil {
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
	}
	tw.tails = nil
	tw.dirs = nil
	tw.watch = nil

	return nil
}

func (tw *TailWatcher) Add(pathname string, maxline int, filter Filter, lines int) (Tail, error) {
	if tw.watch == nil {
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

	tw.mu.Lock()
	defer tw.mu.Unlock()

	// check again with holding lock
	if _, found := tw.tails[absname]; found {
		err = fmt.Errorf("already watching: %s", absname)
		goto ERR_CLOSE
	}
	if refcnt, found := tw.dirs[dirname]; !found {
		err = tw.watch.Add(dirname)
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

	tail = &TailName{
		name:    absname,
		lastp:   pos,
		lines:   q,
		filter:  filter,
		current: q.head,
	}
	tw.tails[absname] = tail

ERR_CLOSE:
	file.Close()
	return tail, err
}

func (tw *TailWatcher) Lookup(pathname string) (Tail, error) {
	if tw.watch == nil {
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
	if tw.watch == nil {
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

	tail.lines.Done()
	delete(tw.tails, absname)

	if refcnt == 1 { // the last one
		if err := tw.watch.Remove(dirname); err != nil {
			if glog.V(1) {
				glog.Infof("fsnotify.Remove(): %s", err)
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
	}
	delete(tw.dirs, dname)
}
