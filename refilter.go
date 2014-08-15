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
var filterFactory func(string, bool) (func(string) bool, error)

type Filter interface {
	Filter(string) bool
	Reload() error
}

type regexpFilter struct {
        name string
	invert bool
	filter func(string)bool
}

func init() {
	filternameExp = regexp.MustCompile("(!?)(.*)")
	filterFactory = joinedExpFilter
}


func RegexpFilter(filtername string)(Filter, error) {
	sm := filternameExp.FindStringSubmatch(filtername)
	if sm == nil {
		return nil, fmt.Errorf("invalid filter name: %s", filtername)
	}

	invert := len(sm[1]) > 0
	filter, err := filterFactory(sm[2], invert)
	if err != nil {
		return  nil, err
	}

	return &regexpFilter{
		name: sm[2],
		filter: filter,
		invert: invert}, nil
}

func (f *regexpFilter) Filter(line string) bool {
	return f.filter(line)
}

func (f *regexpFilter) Reload() error {
	filter, err := filterFactory(f.name, f.invert)
	if err != nil {
		return err
	}
	f.filter = filter
	return nil
}

func (f *regexpFilter) String() string {
	if f.invert {
		return fmt.Sprintf("!%s", f.name)
	}
	return f.name
}

func perExpFilter(path string, inverse bool) (func(string) bool, error) {
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


func joinedExpFilter(path string, inverse bool) (func(string) bool, error) {
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
