package jsonptr

import "strings"

// AppendEscape appends the escaped name to dst and returns it.
// The buffer grows (and so is reallocated) if necessary.
func AppendEscape(dst []byte, name string) []byte {
	if len(name) == 0 {
		return dst
	}

	var shift int
	for i := 0; i < len(name); i++ {
		switch name[i] {
		case '~', '/':
			shift++
		}
	}
	if shift == 0 {
		return append(dst, name...)
	}

	var result []byte
	if cap(dst) >= len(dst)+len(name)+shift {
		result = dst[0 : len(dst)+len(name)+shift]
	} else {
		result = make([]byte, len(dst)+len(name)+shift)
		copy(result, dst)
	}
	b := result[len(dst):]

	for i := len(name) - 1; i >= 0; i-- {
		switch name[i] {
		case '~':
			b[i+shift] = '0'
		case '/':
			b[i+shift] = '1'
		default:
			b[i+shift] = name[i]
			continue
		}
		shift--
		b[i+shift] = '~'
		if shift == 0 {
			if i > 0 {
				copy(b[:i], name[:i])
			}
			break
		}
	}

	return result
}

// EscapeString escapes a property name with JSON Pointer escapes:
//  '~' => `~0`
//  '/' => `~1`
func EscapeString(name string) string {
	var shift int
	for i := 0; i < len(name); i++ {
		switch name[i] {
		case '~', '/':
			shift++
		}
	}
	if shift == 0 {
		return name
	}

	b := make([]byte, len(name)+shift)
	copy(b, name[:])

	for i := len(name) - 1; i >= 0; i-- {
		switch b[i] {
		case '~':
			b[i+shift] = '0'
		case '/':
			b[i+shift] = '1'
		default:
			b[i+shift] = b[i]
			continue
		}
		shift--
		b[i+shift] = '~'
		if shift == 0 {
			break
		}
	}

	return string(b)
}

// Unescape unescapes a property name in place:
//  `~1` => '/'
//  `~0` => '~'
// Any '~' followed by something else (or nothing) is an error ErrSyntax.
// Any '/' is an error ErrSyntax.
func Unescape(b []byte) ([]byte, error) {
	q := -1
Loop:
	for p := 0; p < len(b); p++ {
		switch b[p] {
		case '~':
			q = p
			break Loop
		case '/':
			return nil, ErrUsage
		}
	}

	// Nothing to replace
	if q == -1 {
		return b, nil
	}

	if b[len(b)-1] == '~' {
		return nil, ErrSyntax
	}

	p := q
	for q := p; q < len(b); q++ {
		switch b[q] {
		case '~':
			q++
			switch b[q] {
			case '0':
				b[p] = '~'
			case '1':
				b[p] = '/'
			default:
				return nil, ErrSyntax
			}
		case '/':
			return nil, ErrUsage
		default:
			// Move byte
			b[p] = b[q]
		}
		p++
	}
	return b[:p], nil
}

// UnescapeString unescapes a property name:
//  `~1` => '/'
//  `~0` => '~'
// Any '~' followed by something else (or nothing) is an error ErrSyntax.
// If the input contains '/' the result is undefined (may panic).
func UnescapeString(token string) (string, error) {
	p := strings.IndexByte(token, '~')
	if p == -1 {
		/*
			// Costly check just to detect unlikely bad usage
			if strings.IndexByte(token, '/') >= 0 {
				return "", ErrUsage
			}
		*/
		return token, nil
	}
	if token[len(token)-1] == '~' {
		return "", ErrSyntax
	}

	// Copy to a working buffer
	b := []byte(token)
	for q := p; q < len(token); q++ {
		switch b[q] {
		case '~':
			q++
			switch b[q] {
			case '0':
				b[p] = '~'
			case '1':
				b[p] = '/'
			default:
				return "", ErrSyntax
			}
		case '/':
			return "", ErrUsage
		default:
			// Move byte
			b[p] = b[q]
		}
		p++
	}
	return string(b[:p]), nil
}
