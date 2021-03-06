package main

import (
	"container/list"
	"flag"
	"fmt"
	"github.com/chamaken/lotf"
	"github.com/golang/glog"
	"os"
	"strconv"
	"strings"
)

func usage() {
	fmt.Fprintf(os.Stderr, "Usage of: %s <options> <triplet> [<triplet> <triplet> ...]\n", os.Args[0])
	fmt.Fprintln(os.Stderr, " where options are:")
	flag.PrintDefaults()
	fmt.Fprintln(os.Stderr, " where triplet is colon separated <file>:<filter>:<lines>")
	fmt.Fprintln(os.Stderr, "  file:   target file name")
	fmt.Fprintln(os.Stderr, "  filter: filter file name")
	fmt.Fprintln(os.Stderr, "  lines:  number of last lines to print")
}

type Arg struct {
	// <file name>:<filter name>:<nline>
	fname  string
	filter lotf.Filter
	lines  uint64
}

func main() {
	flag.Usage = usage
	flag.Parse()
	if flag.NArg() < 1 {
		flag.Usage()
		os.Exit(1)
	}

	var err error

	argl := list.New()
	for _, s := range os.Args[1:] {
		args := strings.Split(s, ":")
		arg := &Arg{args[0], nil, 0}
		if len(args) > 1 && len(args[1]) > 0 {
			if arg.filter, err = lotf.RegexpFilter(args[1]); err != nil {
				glog.Fatalf("could not create filter from: %s, error: %s", args[1], err)
			}
		}
		if len(args) > 2 {
			if arg.lines, err = strconv.ParseUint(args[2], 0, 64); err != nil {
				glog.Fatalf("invalid number of lines: %s", args[2])
			}
		}
		argl.PushBack(arg)
	}

	tw, err := lotf.NewTailWatcher()
	if err != nil {
		glog.Fatalf("could not create watcher: %s", err)
	}
	go func() {
		for err = range tw.Error {
			fmt.Printf("ERROR: %s\n", err)
			os.Exit(1)
		}
	}()

	ch := make(chan string)
	for e := argl.Front(); e != nil; e = e.Next() {
		arg := e.Value.(*Arg)
		go func() {
			maxlines := 1
			if arg.lines > 0 {
				maxlines = int(arg.lines)
			}
			tail, err := tw.Add(arg.fname, maxlines, arg.filter, int(arg.lines))
			if err != nil {
				glog.Fatalf("could not add %s to watcher: %s", arg.fname, err)
			}
			for s := tail.WaitNext(); s != nil; s = tail.WaitNext() {
				ch <- *s
			}
		}()
	}

	for line := range ch {
		fmt.Println(line)
	}
}
