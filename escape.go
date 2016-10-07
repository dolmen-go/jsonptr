package jsonptr

import "strings"

// Escape a property name with JSON Pointer escapes:
// "~" => "~0",
// "/" => "~1"
func Escape(name string) string {
	var shift int
	for i := range name {
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

// Unescape unescapes a property name:
//  `~1` => '/'
//  `~0` => '~'
// Any '~' followed by something else (or nothing) is an error
func Unescape(token string) (string, error) {
	p := strings.IndexByte(token, '~')
	if p == -1 {
		return token, nil
	}
	if token[len(token)-1] == '~' {
		return "", ErrSyntax
	}

	// Copy to a working buffer
	b := []byte(token)
	for q := p; q < len(token); q++ {
		if b[q] == '~' {
			q++
			switch b[q] {
			case '0':
				b[p] = '~'
			case '1':
				b[p] = '/'
			default:
				return "", ErrSyntax
			}
		} else {
			// Move byte
			b[p] = b[q]
		}
		p++
	}
	return string(b[:p]), nil
}
