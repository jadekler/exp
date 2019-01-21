// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package xerrors

import (
	"bytes"
	"fmt"
	"io"
	"reflect"
	"strconv"
)

// FormatError calls the FormatError method of err with a errors.Printer
// configured according to s and verb and writes the result to s.
func FormatError(s fmt.State, verb rune, f Formatter) {
	// Assuming this function is only called from the Format method, and given
	// that FormatError takes precedence over Format, it cannot be called from
	// any package that supports errors.Formatter. It is therefore safe to
	// disregard that State may be a specific printer implementation and use one
	// of our choice instead.

	// limitations: does not support printing error as Go struct.

	var (
		sep    = " " // separator before next error
		p      = &state{State: s}
		direct = true
	)

	var err error = f

	switch verb {
	// Note that this switch must match the preference order
	// for ordinary string printing (%#v before %+v, and so on).

	case 'v':
		if s.Flag('#') {
			if stringer, ok := err.(fmt.GoStringer); ok {
				io.WriteString(&p.buf, stringer.GoString())
				goto exit
			}
			// proceed as if it were %v
		} else if s.Flag('+') {
			p.printDetail = true
			sep = "\n  - "
		}
	case 's':
	case 'q', 'x', 'X':
		// Use an intermediate buffer in the rare cases that precision,
		// truncation, or one of the alternative verbs (q, x, and X) are
		// specified.
		direct = false

	default:
		p.buf.WriteString("%!")
		p.buf.WriteRune(verb)
		p.buf.WriteByte('(')
		switch {
		case err != nil:
			p.buf.WriteString(reflect.TypeOf(f).String())
		default:
			p.buf.WriteString("<nil>")
		}
		p.buf.WriteByte(')')
		io.Copy(s, &p.buf)
		return
	}

loop:
	for {
		p.inDetail = false

		switch v := err.(type) {
		case Formatter:
			err = v.FormatError((*printer)(p))
		case fmt.Formatter:
			v.Format(p, 'v')
			break loop
		default:
			io.WriteString(&p.buf, v.Error())
			break loop
		}
		if err == nil {
			break
		}
		if !p.inDetail || !p.printDetail {
			p.buf.WriteByte(':')
		}
		// Strip last newline of detail.
		if bytes.HasSuffix(p.buf.Bytes(), detailSep) {
			p.buf.Truncate(p.buf.Len() - len(detailSep))
		}
		p.buf.WriteString(sep)
		p.inDetail = false
	}

exit:
	width, okW := s.Width()
	prec, okP := s.Precision()

	if !direct || (okW && width > 0) || okP {
		// Construct format string from State s.
		format := []byte{'%'}
		if s.Flag('-') {
			format = append(format, '-')
		}
		if s.Flag('+') {
			format = append(format, '+')
		}
		if s.Flag(' ') {
			format = append(format, ' ')
		}
		if okW {
			format = strconv.AppendInt(format, int64(width), 10)
		}
		if okP {
			format = append(format, '.')
			format = strconv.AppendInt(format, int64(prec), 10)
		}
		format = append(format, string(verb)...)
		fmt.Fprintf(s, string(format), p.buf.String())
	} else {
		io.Copy(s, &p.buf)
	}
}

var detailSep = []byte("\n    ")

// state tracks error printing state. It implements fmt.State.
type state struct {
	fmt.State
	buf bytes.Buffer

	printDetail bool
	inDetail    bool
	needNewline bool
}

func (s *state) Write(b []byte) (n int, err error) {
	if s.printDetail {
		if len(b) == 0 {
			return 0, nil
		}
		if s.inDetail && s.needNewline {
			s.needNewline = false
			s.buf.WriteByte(':')
			s.buf.Write(detailSep)
			if b[0] == '\n' {
				b = b[1:]
			}
		}
		k := 0
		for i, c := range b {
			if c == '\n' {
				s.buf.Write(b[k:i])
				s.buf.Write(detailSep)
				k = i + 1
			}
		}
		s.buf.Write(b[k:])
		s.needNewline = !s.inDetail
	} else if !s.inDetail {
		s.buf.Write(b)
	}
	return len(b), nil
}

// printer wraps a state to implement an xerrors.Printer.
type printer state

func (s *printer) Print(args ...interface{}) {
	if !s.inDetail || s.printDetail {
		fmt.Fprint((*state)(s), args...)
	}
}

func (s *printer) Printf(format string, args ...interface{}) {
	if !s.inDetail || s.printDetail {
		fmt.Fprintf((*state)(s), format, args...)
	}
}

func (s *printer) Detail() bool {
	s.inDetail = true
	return s.printDetail
}