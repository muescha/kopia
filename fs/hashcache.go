package fs

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/kopia/kopia/cas"
)

type hashCacheEntry struct {
	Name     string
	Hash     uint64
	ObjectID cas.ObjectID
}

type hashcacheReader struct {
	scanner   *bufio.Scanner
	nextEntry *hashCacheEntry
	entry0    hashCacheEntry
	entry1    hashCacheEntry
	odd       bool
	first     bool
}

func (hcr *hashcacheReader) open(r io.Reader) error {
	hcr.scanner = bufio.NewScanner(r)
	hcr.nextEntry = nil
	hcr.first = true
	hcr.readahead()
	return nil
}

func (hcr *hashcacheReader) findEntry(relativeName string) *hashCacheEntry {
	for hcr.nextEntry != nil && isLess(hcr.nextEntry.Name, relativeName) {
		hcr.readahead()
	}

	if hcr.nextEntry != nil && relativeName == hcr.nextEntry.Name {
		e := hcr.nextEntry
		hcr.nextEntry = nil
		hcr.readahead()
		return e
	}

	return nil
}

func (hcr *hashcacheReader) readahead() {
	if hcr.scanner != nil {
		if hcr.first {
			hcr.first = false
			if !hcr.scanner.Scan() || hcr.scanner.Text() != "HASHCACHE:v1" {
				hcr.scanner = nil
				return
			}
		}

		hcr.nextEntry = nil
		if hcr.scanner.Scan() {
			var err error
			e := &hashCacheEntry{}
			parts := strings.Split(hcr.scanner.Text(), "|")
			if len(parts) == 3 {
				e.Name = parts[0]
				e.Hash, err = strconv.ParseUint(parts[1], 0, 64)
				e.ObjectID = cas.ObjectID(parts[2])
				if err == nil {
					hcr.nextEntry = e
				}
			}
		}
	}

	if hcr.nextEntry == nil {
		hcr.scanner = nil
	}
}

func (hcr *hashcacheReader) nextManifestEntry() *hashCacheEntry {
	hcr.odd = !hcr.odd
	if hcr.odd {
		return &hcr.entry1
	} else {
		return &hcr.entry0
	}
}

type hashcacheWriter struct {
	writer          io.Writer
	lastNameWritten string
}

func newHashCacheWriter(w io.Writer) *hashcacheWriter {
	hcw := &hashcacheWriter{
		writer: w,
	}
	io.WriteString(w, "HASHCACHE:v1\n")
	return hcw
}

func (hcw *hashcacheWriter) WriteEntry(e hashCacheEntry) error {
	if hcw.lastNameWritten != "" {
		if isLessOrEqual(e.Name, hcw.lastNameWritten) {
			return fmt.Errorf("out-of-order directory entry, previous '%v' current '%v'", hcw.lastNameWritten, e.Name)
		}
		hcw.lastNameWritten = e.Name
	}

	fmt.Fprintf(hcw.writer, "%v|0x%x|%v\n", e.Name, e.Hash, e.ObjectID)

	return nil
}
