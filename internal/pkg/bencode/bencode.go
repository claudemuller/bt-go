package bencode

import (
	"fmt"
	"sort"
	"strings"
)

func Decode(str string) (interface{}, error) {
	lexer := newLexer(str)

	obj, err := lexer.decode()
	if err != nil {
		return nil, fmt.Errorf("error decoding string: %v", err)
	}

	return obj, nil
}

func Encode(data interface{}) (string, error) {
	switch v := data.(type) {
	case string:
		return bencodeStr(v), nil

	case int:
		return fmt.Sprintf("i%de", v), nil

	case map[string]interface{}:
		return bencodeMap(v)

	default:
		return "", fmt.Errorf("unsupported type")
	}
}

func bencodeMap(m map[string]interface{}) (string, error) {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	encPairs := make([]string, 0, len(keys))
	for _, key := range keys {
		val, err := Encode(m[key])
		if err != nil {
			return "", fmt.Errorf("error encoding for key '%s': %v", key, err)
		}

		encPairs = append(encPairs, bencodeStr(key)+val)
	}

	return fmt.Sprintf("d%se", strings.Join(encPairs, "")), nil
}

func bencodeStr(s string) string {
	return fmt.Sprintf("%d:%s", len(s), s)
}
