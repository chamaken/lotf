package main

import (
	"errors"
	"fmt"
	"github.com/chamaken/logger"
	"github.com/chamaken/lotf"
	"os"
	"os/signal"
	"syscall"
)

type resource struct {
	tail   lotf.Tail
	filter lotf.Filter
	ssvr   *StreamServer
	usvr   *DgramServer
}

func sighandler(watcher *lotf.TailWatcher, rcs []resource, errch chan<- error) {
	sigch := make(chan os.Signal, 1)
	signal.Notify(sigch)

	for s := range sigch {
		switch s {
		case syscall.SIGUSR1:
			// reload all filter
			for _, r := range rcs {
				if r.filter != nil {
					if err := r.filter.Reload(); err != nil {
						errch <- err
					}
				}
			}

		case syscall.SIGINT:
			fallthrough
		case syscall.SIGTERM:
			for _, r := range rcs {
				if r.usvr != nil {
					if err := r.usvr.Done(); err != nil {
						errch <- err
					}
				}
				if r.ssvr != nil {
					if err := r.ssvr.Done(); err != nil {
						errch <- err
					}
				}
				if err := watcher.Remove(r.tail.Name()); err != nil {
					errch <- err
				}
			}
			if err := watcher.Close(); err != nil {
				errch <- err
			}
			errch <- errors.New("graceful exit by SIGTERM")
			break

		default:
			logger.Notice("ignore sighanl: %s", s)
		}
	}
}

func main() {
	watcher, err := lotf.NewTailWatcher()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error - could not create watcher: %s\n", err)
	}

	// logger has been set up in parseFlags()
	flags, nlines, err := parseFlags()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}

	errch := make(chan error, 512) // XXX: magic number
	rcs := make([]resource, len(flags))
	for i, rc := range flags {
		if rc.tcpaddr == nil && rc.udpaddr == nil {
			fmt.Fprintf(os.Stderr, "error - no inet4 server specified\n")
			os.Exit(1)
		}

		logger.Info("adding watch - path: %s, filter: %s", rc.filename, rc.filter)
		if rcs[i].tail, err = watcher.Add(rc.filename, nlines, rc.filter, rc.buflines); err != nil {
			logger.Fatal("could not watch: %s\n", err)
		}
		logger.Info("watch added - path: %s, filter: %s", rc.filename, rc.filter)

		if rc.tcpaddr != nil {
			logger.Info("starting TCP service - addr: %v)", rc.tcpaddr)
			if rcs[i].ssvr, err = NewTCPServer(rcs[i].tail, rc.tcpaddr); err != nil {
				fmt.Fprintf(os.Stderr, "error - could not start TCP service: %s\n", err)
				os.Exit(1)
			}
			go rcs[i].ssvr.Run(errch)
		}
		if rc.udpaddr != nil {
			logger.Info("starting UDP service - addr: %v", rc.udpaddr)
			if rcs[i].usvr, err = NewUDPServer(rcs[i].tail, rc.udpaddr); err != nil {
				fmt.Fprintf(os.Stderr, "error - could not start UDP service: %s\n", err)
				os.Exit(1)
			}
			go rcs[i].usvr.Run(errch)
		}
	}

	// signal handler
	go sighandler(watcher, rcs, errch)

	// daemonize?
	for err := range errch {
		logger.Error("%s", err)
		break
	}
}
