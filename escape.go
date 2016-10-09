package jsonptr

import "strings"

// Escape a property name with JSON Pointer escapes:
// "~" => "~0",
// "/" => "~1"
func Escape(name string) string {
	var shift int
	for i := len(name) - 1; i >= 0; i-- {
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

var ErrUsage = stringError("invalid use of json.Unescape on string with '/'")

// Unescape unescapes a property name:
//  `~1` => '/'
//  `~0` => '~'
// Any '~' followed by something else (or nothing) is an error ErrSyntax
// Any '/' is an error ErrUsage
func Unescape(token string) (string, error) {
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
