package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
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

func plainTextRepresentation(context []Message, useColor bool) string {
	var maybeBoldFgWhiteString func(string, ...interface{}) string
	var maybeCyanString func(string, ...interface{}) string

	if useColor {
		maybeBoldFgWhiteString = color.Set(color.Bold, color.FgWhite).Sprintf
		maybeCyanString = color.CyanString
	} else {
		maybeBoldFgWhiteString = fmt.Sprintf
		maybeCyanString = fmt.Sprintf
	}
	var result bytes.Buffer
	for _, msg := range context {
		result.WriteString(fmt.Sprintf("%v%v%v\n", maybeCyanString("["), maybeBoldFgWhiteString("%v", msg.Role), maybeCyanString("]")))
		result.WriteString(fmt.Sprintf("%v\n\n", msg.Content))
	}
	return result.String()
}

func parseUncoloredPlainTextRepresentation(repr string) ([]Message, error) {
	context := []Message{}
	var currentMessageContent bytes.Buffer
	currentRole := ""
	for _, line := range strings.Split(repr, "\n") {
		line = strings.TrimSpace(line)
		if len(line) >= 3 && line[0] == '[' && line[len(line)-1] == ']' {
			if currentRole != "" {
				context = append(context, Message{Role: currentRole, Content: strings.TrimSpace(currentMessageContent.String())})
			}
			currentRole = line[1 : len(line)-1]
			currentMessageContent.Reset()
			if !isRoleValid(currentRole) {
				return nil, fmt.Errorf("invalid role: %v", currentRole)
			}
		} else if currentRole != "" && len(line) > 0 {
			currentMessageContent.WriteString(line)
			currentMessageContent.WriteString("\n")
		} else if line != "" {
			return nil, fmt.Errorf("expected a [role], found %v", line)
		}
	}
	if currentMessageContent.Len() > 0 {
		context = append(context, Message{Role: currentRole, Content: strings.TrimSpace(currentMessageContent.String())})
	}
	return context, nil
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
