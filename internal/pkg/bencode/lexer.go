package bencode

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

type lexer struct {
	input        string
	position     int  // the current char
	readPosition int  // the current reading position i.e. position after current char
	ch           byte // the current char
}

func newLexer(input string) *lexer {
	l := &lexer{input: input}
	l.readChar()

	return l
}

func (l *lexer) decode() (interface{}, error) {
	for l.ch != 0 {
		switch {
		case l.ch == 'i':
			return l.readInt()

		case unicode.IsDigit(rune(l.ch)):
			return l.readStr()

		case l.ch == 'l':
			return l.readList()

		case l.ch == 'd':
			return l.readDict()

		default:
			return "", fmt.Errorf("unsupported type: %c", l.ch)
		}
	}

	return nil, fmt.Errorf("there was nothing to decode :(")
}

func (l *lexer) readChar() {
	l.ch = 0
	if l.readPosition < len(l.input) {
		l.ch = l.input[l.readPosition]
	}

	l.position = l.readPosition
	l.readPosition++
}

func (l *lexer) readStr() (string, error) {
	startPos := l.position

	l.readChar()

	// Error checking here
	for l.ch != ':' {
		l.readChar()
	}

	lengthStr := l.input[startPos:l.position]

	length, err := strconv.Atoi(lengthStr)
	if err != nil {
		return "", err
	}

	var strBuf strings.Builder

	l.readChar()

	for i := startPos; i < length+startPos; i++ {
		strBuf.WriteByte(l.ch)
		l.readChar()
	}

	return strBuf.String(), nil
}

func (l *lexer) readInt() (int, error) {
	l.readChar()

	startPos := l.position

	for l.ch != 'e' {
		if l.ch == '-' || unicode.IsDigit(rune(l.ch)) {
			l.readChar()
		} else {
			return 0, fmt.Errorf("invalid int")
		}
	}

	val, err := strconv.Atoi(l.input[startPos:l.position])
	if err != nil {
		return 0, fmt.Errorf("error converting val to int: %v", err)
	}

	l.readChar()

	return val, nil
}

func (l *lexer) readDict() (interface{}, error) {
	l.readChar()

	res := map[string]interface{}{}

	for l.ch != 'e' {
		k, err := l.decode()
		if err != nil {
			return nil, err
		}

		v, err := l.decode()
		if err != nil {
			return nil, err
		}

		key, ok := k.(string)
		if !ok {
			return nil, fmt.Errorf("invalid key format")
		}

		res[key] = v
	}

	return res, nil
}

func (l *lexer) readList() ([]interface{}, error) {
	l.readChar()

	res := []interface{}{}

	for l.ch != 'e' {
		v, err := l.decode()
		if err != nil {
			return nil, err
		}

		res = append(res, v)
	}

	return res, nil
}
