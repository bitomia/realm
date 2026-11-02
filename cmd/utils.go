package cmd

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

func prettyJSON(v any, excludeFields ...string) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		panic(err)
	}

	excludeMap := make(map[string]bool)
	for _, field := range excludeFields {
		excludeMap[field] = true
	}

	lines := bytes.SplitSeq(b, []byte{'\n'})
	for line := range lines {
		shouldSkip := false
		for field := range excludeMap {
			if bytes.Contains(line, []byte(field+":")) || bytes.Contains(line, []byte(`"`+field+`"`)) {
				shouldSkip = true
				break
			}
		}
		if shouldSkip {
			continue
		}

		line = bytes.ReplaceAll(line, []byte(`"`), []byte(""))
		line = bytes.ReplaceAll(line, []byte(","), []byte(""))
		line = bytes.TrimLeft(line, "{}")
		line = bytes.TrimRight(line, "{}")

		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}

		fmt.Println(string(line))
	}
}

func confirm(question string, in io.Reader, out io.Writer) (bool, error) {
	fmt.Fprintf(out, "%s (y/N): ", question)
	response, err := bufio.NewReader(in).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, fmt.Errorf("failed to read confirmation: %w", err)
	}
	if errors.Is(err, io.EOF) {
		fmt.Fprintln(out)
	}

	response = strings.TrimSpace(strings.ToLower(response))
	return response == "y" || response == "yes", nil
}
