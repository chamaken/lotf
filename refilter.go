package lotf

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
)

var filternameExp *regexp.Regexp

type Filter struct {
        Name string
        Func func(string) bool
}

func init() {
	filternameExp = regexp.MustCompile("(!?)(.*)")
}


func CreateReFilter(filtername string)(*Filter, error) {
	sm := filternameExp.FindStringSubmatch(filtername)
	if sm == nil {
		return nil, fmt.Errorf("invalid filter name: %s", filtername)
	}

	filter, err := JoinedExpFilter(sm[2], len(sm[1]) > 0)
	if err != nil {
		return  nil, err
	}

	return &Filter{filtername, filter}, nil
}


func PerExpFilter(path string, inverse bool) (func(string) bool, error) {
	var err error

	refile, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer refile.Close()

	regexps := []*regexp.Regexp{}
	r := bufio.NewReader(refile)

LOOP:
	for {
		line, err := r.ReadString(byte('\n'))

		if err != nil && err != io.EOF { return nil, err }
		if len(line) == 0 && err == io.EOF { break }

		nlidx := strings.LastIndex(line, "\n")
		switch {
		case nlidx == 0: { continue LOOP } // ignore empty line
		case nlidx < 0: // do nothing, may be lastline not ended with \n
		default:
			line = line[:len(line) - 1]
		}

		rexp, err := regexp.Compile(line)
		if err != nil {
			return nil, err
		}
		regexps = append(regexps, rexp)
	}

	return func(s string) bool {
		for _, re := range regexps {
			if (re.FindStringIndex(s) != nil) != !inverse {
				return false
			}
		}
		return true
	}, nil
}


func JoinedExpFilter(path string, inverse bool) (func(string) bool, error) {
	var err error

	refile, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer refile.Close()

	// regbuf := new(bytes.Buffer)
	regbuf := bytes.NewBufferString("(")
	r := bufio.NewReader(refile)

LOOP:
	for {
		line, err := r.ReadString(byte('\n'))

		if err != nil && err != io.EOF { return nil, err }
		if len(line) == 0 && err == io.EOF { break }

		nlidx := strings.LastIndex(line, "\n")
		switch {
		case nlidx == 0: { continue LOOP } // ignore empty line
		case nlidx < 0: // do nothing, may be lastline not ended with \n
		default:
			line = line[:len(line) - 1]
		}

		if _, err := regbuf.WriteString(line + "|"); err != nil {
			return nil, err
		}
	}

	b := regbuf.Bytes()
	b[len(b) - 1] = byte(')')
	joinedexp := string(b)
	
	re, err := regexp.Compile(joinedexp)
	if err != nil {	return nil, err	}

	return func(s string) bool {
		return (re.FindStringIndex(s) != nil) == !inverse
	}, nil
}
