package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

func parseContextFile(path string) ([]Message, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var unmarshaled []Message
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		return nil, err
	}
	for idx, msg := range unmarshaled {
		if !isRoleValid(msg.Role) {
			return nil, fmt.Errorf("%v: message #%v (starting from zero) has an invalid \"role\" attribute", path, idx)
		}
	}
	return unmarshaled, nil
}

func writeContextFile(path string, context []Message) error {
	marshaled, err := json.MarshalIndent(context, "", "\t")
	if err != nil {
		return err
	}
	return os.WriteFile(path, marshaled, 0660)
}

func isRoleValid(role string) bool {
	return role == "user" || role == "assistant" || role == "system"
}

func textWrap(text string, maxLineLen int) []string {
	words := strings.Fields(text)
	var lines []string
	var currentLine bytes.Buffer

	for _, word := range words {
		if len(currentLine.String())+len(word)+1 > maxLineLen {
			lines = append(lines, currentLine.String())
			currentLine.Reset()
			currentLine.WriteString(word)
		} else {
			if currentLine.Len() != 0 {
				currentLine.WriteString(" ")
			}
			currentLine.WriteString(word)
		}
	}

	if currentLine.Len() != 0 {
		lines = append(lines, currentLine.String())
	}

	return lines
}
