package utils

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode"
)

// reader for the CLF log file format
type CLFReader struct {
	format       []string
	messageTypes map[string][]string
}

func (r *CLFReader) processMeta(line string) error {
	if strings.HasPrefix(line, "message formats defined") {
		// ignored this
		return nil
	}

	if len(r.format) == 0 {
		r.format = clfSplit(line)
		return nil
	}

	if r.messageTypes == nil {
		r.messageTypes = map[string][]string{}
	}

	pcs := clfSplit(line)
	r.messageTypes[pcs[0]] = pcs[1:]

	numArrays := 0
	for _, s := range pcs {
		if s[0] == '[' {
			numArrays++
		}
	}

	if numArrays > 1 {
		return fmt.Errorf("too many arrays (%d) in (%s)", numArrays, pcs)
	}

	return nil
}

func (r *CLFReader) combineFormats(sub []string) []string {
	n := []string{}

	for _, s := range r.format {
		if s == "[message contents]" {
			n = append(n, sub...)
		} else {
			n = append(n, s)
		}
	}

	return n
}

func (r *CLFReader) processPiece(p string) interface{} {
	for _, c := range p {
		if c == '.' || c == '-' || unicode.IsDigit(c) {
			continue
		}
		return p
	}
	x, err := strconv.ParseFloat(p, 64)
	if err == nil {
		return x
	}
	return p
}

func (r *CLFReader) processLine(line string) (map[string]interface{}, error) {
	if len(line) == 0 {
		return nil, nil
	}

	if line[0] == '#' {
		return nil, r.processMeta(line[2:])
	}

	pcs := strings.Split(line, " ")
	msgFormat := r.messageTypes[pcs[0]]
	if len(msgFormat) == 0 {
		return nil, fmt.Errorf("unknown type %s", pcs[0])
	}

	msgFormat = r.combineFormats(msgFormat)

	m := map[string]interface{}{}

	offset := 0
	for idx, key := range msgFormat {
		if key[0] == '[' {
			v := []interface{}{}

			for ; offset <= (len(pcs) - len(msgFormat)); offset++ {
				v = append(v, r.processPiece(pcs[idx+offset]))
			}

			key = key[1:]
			key = key[0 : len(key)-1]
			m[key] = v
			offset--
			continue
		}
		v := ""
		if idx < len(pcs) {
			v = pcs[idx+offset]
		}
		m[key] = r.processPiece(v)
	}

	return m, nil
}

func (r *CLFReader) Process(reader *bufio.Reader, f func(data map[string]interface{}) error) error {
	for {
		line, err := reader.ReadString('\n')
		if err == io.EOF {
			return nil
		} else if err != nil {
			return err
		}

		line = strings.TrimSpace(line)

		res, err := r.processLine(line)
		if err != nil {
			return err
		}

		if res == nil {
			continue
		}

		err = f(res)
		if err != nil {
			return err
		}
	}
}

func clfSplit(s string) []string {
	pcs := strings.Split(s, " ")

	n := []string{}
	inArray := false
	for _, s := range pcs {

		if inArray {
			n[len(n)-1] = n[len(n)-1] + " " + s
			if s[len(s)-1] == ']' {
				inArray = false
			}
			continue
		}

		if s[0] != '[' {
			n = append(n, s)
			continue
		}
		n = append(n, s)
		inArray = s[len(s)-1] != ']'
	}
	return n
}
