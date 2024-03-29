// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package json

import (
	"bytes"
	"io"
)

// Compact appends to dst the JSON-encoded src with
// insignificant space characters elided.
func Compact(dst *bytes.Buffer, src []byte) error {
	return compactWithRevert(dst, src, false)
}

func compactWithRevert(dst *bytes.Buffer, src []byte, escape bool) error {
	origLen := dst.Len()
	if err := compact(dst, src, escape); err != nil {
		dst.Truncate(origLen)
		return err
	}
	return nil
}

func compact(dst writer, src []byte, escape bool) error {
	var scan scanner
	scan.reset()
	start := 0
	for i, c := range src {
		if escape && (c == '<' || c == '>' || c == '&') {
			if start < i {
				if _, err := dst.Write(src[start:i]); err != nil {
					return err
				}
			}
			if _, err := dst.WriteString(`\u00`); err != nil {
				return err
			}
			if err := dst.WriteByte(hex[c>>4]); err != nil {
				return err
			}
			if err := dst.WriteByte(hex[c&0xF]); err != nil {
				return err
			}
			start = i + 1
		}
		// Convert U+2028 and U+2029 (E2 80 A8 and E2 80 A9).
		if c == 0xE2 && i+2 < len(src) && src[i+1] == 0x80 && src[i+2]&^1 == 0xA8 {
			if start < i {
				if _, err := dst.Write(src[start:i]); err != nil {
					return err
				}
			}
			if _, err := dst.WriteString(`\u202`); err != nil {
				return err
			}
			if err := dst.WriteByte(hex[src[i+2]&0xF]); err != nil {
				return err
			}
			start = i + 3
		}
		v := scan.step(&scan, c)
		if v >= scanSkipSpace {
			if v == scanError {
				break
			}
			if start < i {
				if _, err := dst.Write(src[start:i]); err != nil {
					return err
				}
			}
			start = i + 1
		}
	}
	if scan.eof() == scanError {
		return scan.err
	}
	if start < len(src) {
		if _, err := dst.Write(src[start:]); err != nil {
			return err
		}
	}
	return nil
}

func newline(dst writer, prefix, indent string, depth int) error {
	if err := dst.WriteByte('\n'); err != nil {
		return err
	}
	if _, err := dst.WriteString(prefix); err != nil {
		return err
	}
	for i := 0; i < depth; i++ {
		if _, err := dst.WriteString(indent); err != nil {
			return err
		}
	}
	return nil
}

// Indent appends to dst an indented form of the JSON-encoded src.
// Each element in a JSON object or array begins on a new,
// indented line beginning with prefix followed by one or more
// copies of indent according to the indentation nesting.
// The data appended to dst does not begin with the prefix nor
// any indentation, to make it easier to embed inside other formatted JSON data.
// Although leading space characters (space, tab, carriage return, newline)
// at the beginning of src are dropped, trailing space characters
// at the end of src are preserved and copied to dst.
// For example, if src has no trailing spaces, neither will dst;
// if src ends in a trailing newline, so will dst.
func Indent(dst *bytes.Buffer, src []byte, prefix, indent string) error {
	origLen := dst.Len()
	var scan scanner
	scan.reset()
	w := &indentWriter{
		dst:    dst,
		prefix: prefix,
		indent: indent,
		scan:   &scan,
	}
	if _, err := w.Write(src); err != nil {
		dst.Truncate(origLen)
		return err
	}
	return nil
}

// IndentWriter wraps w, re-indenting the data written to it, according to
// prefix and indent. If any parsing error occurs, it will be returned on the
// next call to Write() on the returned io.Writer.
func IndentWriter(w io.Writer, prefix, indent string) io.Writer {
	dst, ok := w.(writer)
	if !ok {
		dst = &convertWriter{w}
	}
	var scan scanner
	scan.reset()
	return &indentWriter{
		dst:    dst,
		prefix: prefix,
		indent: indent,
		scan:   &scan,
	}
}

type indentWriter struct {
	dst        writer
	prefix     string
	indent     string
	depth      int
	scan       *scanner
	needIndent bool
}

func (w *indentWriter) Write(src []byte) (int, error) {
	var n int
	for _, c := range src {
		n++
		w.scan.bytes++
		v := w.scan.step(w.scan, c)
		if v == scanSkipSpace {
			continue
		}
		if v == scanError {
			break
		}
		if w.needIndent && v != scanEndObject && v != scanEndArray {
			w.needIndent = false
			w.depth++
			if err := newline(w.dst, w.prefix, w.indent, w.depth); err != nil {
				return n, err
			}
		}

		// Emit semantically uninteresting bytes
		// (in particular, punctuation in strings) unmodified.
		if v == scanContinue {
			if err := w.dst.WriteByte(c); err != nil {
				return n, err
			}
			continue
		}

		// Add spacing around real punctuation.
		switch c {
		case '{', '[':
			// delay indent so that empty object and array are formatted as {} and [].
			w.needIndent = true
			if err := w.dst.WriteByte(c); err != nil {
				return n, err
			}

		case ',':
			if err := w.dst.WriteByte(c); err != nil {
				return n, err
			}
			if err := newline(w.dst, w.prefix, w.indent, w.depth); err != nil {
				return n, err
			}

		case ':':
			if err := w.dst.WriteByte(c); err != nil {
				return n, err
			}
			if err := w.dst.WriteByte(' '); err != nil {
				return n, err
			}

		case '}', ']':
			if w.needIndent {
				// suppress indent in empty object/array
				w.needIndent = false
			} else {
				w.depth--
				if err := newline(w.dst, w.prefix, w.indent, w.depth); err != nil {
					return n, err
				}
			}
			if err := w.dst.WriteByte(c); err != nil {
				return n, err
			}

		default:
			if err := w.dst.WriteByte(c); err != nil {
				return n, err
			}
		}
	}
	if w.scan.eof() == scanError {
		return n, w.scan.err
	}
	return n, nil
}
