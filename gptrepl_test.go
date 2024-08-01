package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
)

type MockPrinter struct {
	info bytes.Buffer
	warn bytes.Buffer
	err  bytes.Buffer
}

func (mp *MockPrinter) Print(format string, a ...interface{}) {
	mp.info.WriteString(fmt.Sprintf(format, a...))
}

func (mp *MockPrinter) PrintWarning(format string, a ...interface{}) {
	mp.warn.WriteString(fmt.Sprintf(format, a...))
}

func (mp *MockPrinter) PrintError(format string, a ...interface{}) {
	mp.err.WriteString(fmt.Sprintf(format, a...))
}

func (mp *MockPrinter) expectNoOutput(t *testing.T) {
	if mp.info.Len() > 0 {
		t.Fatalf("unexpected output: %v", mp.info.String())
	}
}

func (mp *MockPrinter) expectNoErrors(t *testing.T) {
	if mp.err.Len() > 0 {
		t.Fatalf("unexpected error: %v", mp.err.String())
	}
}

func (mp *MockPrinter) expectNoWarnings(t *testing.T) {
	if mp.warn.Len() > 0 {
		t.Fatalf("unexpected warning: %v", mp.warn.String())
	}
}

func makeTestPrinter() *MockPrinter {
	return &MockPrinter{}
}

type MockCompletionAPI struct {
	receivedContext []Message
	contentToSend   []CompletionDelta
	err             error
}

func (mca *MockCompletionAPI) SendContext(context []Message) (<-chan CompletionDelta, error) {
	if mca.err != nil {
		return nil, mca.err
	}
	mca.receivedContext = context
	out := make(chan CompletionDelta, len(mca.contentToSend))
	for _, cd := range mca.contentToSend {
		out <- cd
	}
	return out, nil
}

func (mca *MockCompletionAPI) expectNoSentContent(t *testing.T) {
	if len(mca.receivedContext) > 0 {
		t.Fatalf("expected not to send context, but got %v", mca.receivedContext)
	}
}

func makeTestCompletionAPI() *MockCompletionAPI {
	return &MockCompletionAPI{
		receivedContext: nil,
		contentToSend:   []CompletionDelta{{"One", nil}, {"Two", nil}, {"Three", nil}, {"", io.EOF}},
	}
}

func makeTestApp() (App, *MockPrinter, *MockCompletionAPI) {
	mp := makeTestPrinter()
	mca := makeTestCompletionAPI()
	model := "test-model"
	apikey := "sk-test"
	app := App{
		context:               make([]Message, 0),
		model:                 &model,
		slashCommandsDisabled: false,
		quiet:                 true,
		apiKey:                &apikey,
		commandHandlers:       make(map[string]Command),
		autosaveFilePath:      "",
		printer:               mp,
		capi:                  mca,
	}
	return app, mp, mca
}

func temporaryFilePath() string {
	file, err := os.CreateTemp("", "gptrepl_test")
	if err != nil {
		panic(err)
	}
	err = file.Close()
	if err != nil {
		panic(err)
	}
	return file.Name()
}

func temporaryJsonFileWithMessages(ctx []Message) string {
	file := temporaryFilePath()
	err := writeContextFile(file, ctx)
	if err != nil {
		panic(err)
	}
	return file
}

func readStringFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	return string(data)
}

type MockReadliner struct {
	lines   []string
	err     error
	current int
}

func (mr *MockReadliner) Readline() (string, error) {
	if mr.err != nil {
		return "", mr.err
	}
	if mr.current >= len(mr.lines) {
		return "", io.EOF
	}
	ret := mr.lines[mr.current]
	mr.current++
	return ret, nil
}

func TestAppMainEOF(t *testing.T) {
	mr := &MockReadliner{err: io.EOF}
	a, p, c := makeTestApp()
	if a.appMain(mr) {
		t.Fatalf("appMain returned true")
	}
	p.expectNoOutput(t)
	p.expectNoWarnings(t)
	p.expectNoErrors(t)
	c.expectNoSentContent(t)
}

func TestUnknownCommandBehavior(t *testing.T) {
	mr := &MockReadliner{lines: []string{"/ajiwmcn"}}
	a, p, c := makeTestApp()
	if !a.appMain(mr) {
		t.Fatalf("appMain returned false")
	}
	c.expectNoSentContent(t)
	if !strings.Contains(p.err.String(), "unknown command") {
		t.Fatalf("expected %v to contain 'unknown command'", p.err.String())
	}
}

func TestEmptyLine(t *testing.T) {
	mr := &MockReadliner{lines: []string{"    \t  \t   "}}
	a, p, c := makeTestApp()
	if !a.appMain(mr) {
		t.Fatalf("appMain returned false")
	}
	c.expectNoSentContent(t)
	p.expectNoOutput(t)
	p.expectNoErrors(t)
	p.expectNoWarnings(t)
}

func TestHelpUsesCommandHandlers(t *testing.T) {
	mr := &MockReadliner{lines: []string{"/help"}}
	a, p, c := makeTestApp()
	a.registerCommandHandlers()
	a.commandHandlers["testCommand"] = NewCommand(nil, "TEST DESCRIPTION", [][]string{{"test argument"}})
	if !a.appMain(mr) {
		t.Fatalf("appMain returned false")
	}
	p.expectNoErrors(t)
	p.expectNoWarnings(t)
	c.expectNoSentContent(t)
	if !strings.Contains(p.info.String(), "TEST DESCRIPTION") {
		t.Fatalf("description not included in help message")
	}
	if !strings.Contains(p.info.String(), "test argument") {
		t.Fatalf("argument not included in help message")
	}
}

func TestHelpWarnsWhenArgumentsArePassed(t *testing.T) {
	mr := &MockReadliner{lines: []string{"/help test"}}
	a, p, c := makeTestApp()
	a.registerCommandHandlers()
	if !a.appMain(mr) {
		t.Fatalf("appMain returned false")
	}
	p.expectNoErrors(t)
	c.expectNoSentContent(t)
	if !strings.Contains(p.warn.String(), "takes no arguments") {
		t.Fatalf("expected warning message to contain 'text no arguments'")
	}
	if !strings.Contains(p.info.String(), "/help") {
		t.Fatalf("'/help' not included in help message")
	}
	if !strings.Contains(p.info.String(), "help page") {
		t.Fatalf("'help page' not included in help message")
	}
}

func assertCommandHasWrongNumberOfArguments(t *testing.T, command string) {
	mr := &MockReadliner{lines: []string{command}}
	a, p, c := makeTestApp()
	a.registerCommandHandlers()
	if !a.appMain(mr) {
		t.Fatalf("appMain returned false")
	}
	p.expectNoWarnings(t)
	p.expectNoOutput(t)
	c.expectNoSentContent(t)
	if !strings.Contains(p.err.String(), "argument") {
		t.Fatalf("expected an error message containing 'argument', got %v", p.err.String())
	}
}

func assertErrorMessageOnInvalidPath(t *testing.T, command string) {
	mr := &MockReadliner{lines: []string{command + " /dwadm/diawcjci/venoacja/dawcdno.json"}}
	a, p, c := makeTestApp()
	a.registerCommandHandlers()
	if !a.appMain(mr) {
		t.Fatalf("appMain returned false")
	}
	p.expectNoWarnings(t)
	p.expectNoOutput(t)
	c.expectNoSentContent(t)
	if p.err.Len() == 0 {
		t.Fatalf("expected an error message")
	}
}

func assertContextEquals(t *testing.T, actual []Message, expect []Message) {
	for i, msg := range actual {
		if msg.Role != expect[i].Role {
			t.Fatalf("role mismatch at position %v. expected %v, got %v", i, expect[i].Role, msg.Role)
		}
		if msg.Content != expect[i].Content {
			t.Fatalf("content mismatch at position %v. expected %v, got %v", i, expect[i].Content, msg.Content)
		}
	}
}

func TestSaveCommandNoArguments(t *testing.T) {
	assertCommandHasWrongNumberOfArguments(t, "/save")
}

func TestSaveCommandInvalidPath(t *testing.T) {
	assertErrorMessageOnInvalidPath(t, "/save")
}

func TestSaveCommand(t *testing.T) {
	file := temporaryFilePath()
	defer os.Remove(file)
	mr := &MockReadliner{lines: []string{"/save " + file}}
	a, p, c := makeTestApp()
	a.registerCommandHandlers()
	a.appendToContext(Message{Role: "user", Content: "test content"})
	a.appendToContext(Message{Role: "system", Content: "abcdef"})
	if !a.appMain(mr) {
		t.Fatalf("appMain returned false")
	}
	p.expectNoWarnings(t)
	p.expectNoOutput(t)
	p.expectNoErrors(t)
	c.expectNoSentContent(t)
	data := readStringFile(file)
	for _, s := range []string{"user", "test content", "system", "abcdef"} {
		if !strings.Contains(data, s) {
			t.Fatalf("expected saved data to contain '%v'", s)
		}
	}
}

func TestReplaceFromCommandNoArguments(t *testing.T) {
	assertCommandHasWrongNumberOfArguments(t, "/replacefrom")
}

func TestReplaceFromCommandInvalidPath(t *testing.T) {
	assertErrorMessageOnInvalidPath(t, "/replacefrom")
}

func TestReplaceFromCommand(t *testing.T) {
	testMessages := []Message{
		{Role: "user", Content: "a"},
		{Role: "assistant", Content: "b"},
	}
	file := temporaryJsonFileWithMessages(testMessages)
	defer os.Remove(file)
	mr := &MockReadliner{lines: []string{"/replacefrom " + file}}
	a, p, c := makeTestApp()
	a.registerCommandHandlers()
	a.context = []Message{{Role: "system", Content: "test"}}
	if !a.appMain(mr) {
		t.Fatalf("appMain returned false")
	}
	p.expectNoErrors(t)
	p.expectNoOutput(t)
	p.expectNoWarnings(t)
	c.expectNoSentContent(t)
	assertContextEquals(t, a.context, testMessages)
}

func TestAppendFromCommandNoArguments(t *testing.T) {
	assertCommandHasWrongNumberOfArguments(t, "/appendfrom")
}

func TestAppendFromCommandInvalidPath(t *testing.T) {
	assertErrorMessageOnInvalidPath(t, "/appendfrom")
}

func TestAppendFromCommand(t *testing.T) {
	testMessages := []Message{
		{Role: "user", Content: "a"},
		{Role: "assistant", Content: "b"},
	}
	expect := []Message{
		{Role: "system", Content: "test"},
		{Role: "user", Content: "a"},
		{Role: "assistant", Content: "b"},
	}
	file := temporaryJsonFileWithMessages(testMessages)
	defer os.Remove(file)
	mr := &MockReadliner{lines: []string{"/appendfrom " + file}}
	a, p, c := makeTestApp()
	a.registerCommandHandlers()
	a.context = []Message{{Role: "system", Content: "test"}}
	if !a.appMain(mr) {
		t.Fatalf("appMain returned false")
	}
	p.expectNoErrors(t)
	p.expectNoOutput(t)
	p.expectNoWarnings(t)
	c.expectNoSentContent(t)
	assertContextEquals(t, a.context, expect)
}

func TestPrependFromCommandNoArguments(t *testing.T) {
	assertCommandHasWrongNumberOfArguments(t, "/prependfrom")
}

func TestPrependFromCommandInvalidPath(t *testing.T) {
	assertErrorMessageOnInvalidPath(t, "/prependfrom")
}

func TestPrependFromCommand(t *testing.T) {
	testMessages := []Message{
		{Role: "user", Content: "a"},
		{Role: "assistant", Content: "b"},
	}
	expect := []Message{
		{Role: "user", Content: "a"},
		{Role: "assistant", Content: "b"},
		{Role: "system", Content: "test"},
	}
	file := temporaryJsonFileWithMessages(testMessages)
	defer os.Remove(file)
	mr := &MockReadliner{lines: []string{"/prependfrom " + file}}
	a, p, c := makeTestApp()
	a.registerCommandHandlers()
	a.context = []Message{{Role: "system", Content: "test"}}
	if !a.appMain(mr) {
		t.Fatalf("appMain returned false")
	}
	p.expectNoErrors(t)
	p.expectNoOutput(t)
	p.expectNoWarnings(t)
	c.expectNoSentContent(t)
	assertContextEquals(t, a.context, expect)
}

func TestClearCommandErrorsWithArguments(t *testing.T) {
	expect := []Message{{Role: "system", Content: "test"}}
	mr := &MockReadliner{lines: []string{"/clear cnwdji"}}
	a, p, c := makeTestApp()
	a.registerCommandHandlers()
	a.context = []Message{{Role: "system", Content: "test"}}
	if !a.appMain(mr) {
		t.Fatalf("appMain returned false")
	}
	p.expectNoOutput(t)
	p.expectNoWarnings(t)
	c.expectNoSentContent(t)
	if !strings.Contains(p.err.String(), "argument") {
		t.Fatalf("expected error message to contain 'argument'")
	}
	assertContextEquals(t, a.context, expect)
}

func TestClearCommand(t *testing.T) {
	mr := &MockReadliner{lines: []string{"/clear"}}
	a, p, c := makeTestApp()
	a.registerCommandHandlers()
	a.context = []Message{{Role: "system", Content: "test"}}
	if !a.appMain(mr) {
		t.Fatalf("appMain returned false")
	}
	p.expectNoOutput(t)
	p.expectNoWarnings(t)
	p.expectNoErrors(t)
	c.expectNoSentContent(t)
	if len(a.context) > 0 {
		t.Fatalf("expected empty context, got %v", a.context)
	}
}

func TestPrintWarnsWhenArgumentsArePassed(t *testing.T) {
	mr := &MockReadliner{lines: []string{"/print test"}}
	a, p, c := makeTestApp()
	a.registerCommandHandlers()
	a.context = []Message{{Role: "system", Content: "test content"}}
	if !a.appMain(mr) {
		t.Fatalf("appMain returned false")
	}
	p.expectNoErrors(t)
	c.expectNoSentContent(t)
	if !strings.Contains(p.warn.String(), "takes no arguments") {
		t.Fatalf("expected warning message to contain 'text no arguments'")
	}
	if !strings.Contains(p.info.String(), "system") {
		t.Fatalf("'system' not included in printed data")
	}
	if !strings.Contains(p.info.String(), "test content") {
		t.Fatalf("'test content' not included in printed data")
	}
}

func TestPrintCommand(t *testing.T) {
	mr := &MockReadliner{lines: []string{"/print"}}
	a, p, c := makeTestApp()
	a.registerCommandHandlers()
	a.context = []Message{{Role: "system", Content: "test content"}}
	if !a.appMain(mr) {
		t.Fatalf("appMain returned false")
	}
	p.expectNoErrors(t)
	p.expectNoWarnings(t)
	c.expectNoSentContent(t)
	if !strings.Contains(p.info.String(), "system") {
		t.Fatalf("'system' not included in printed data")
	}
	if !strings.Contains(p.info.String(), "test content") {
		t.Fatalf("'test content' not included in printed data")
	}
}

func TestAppendCommandNoArguments(t *testing.T) {
	assertCommandHasWrongNumberOfArguments(t, "/append")
}

func TestAppendCommandInvalidRole(t *testing.T) {
	mr := &MockReadliner{lines: []string{"/append invalid test message"}}
	a, p, c := makeTestApp()
	a.registerCommandHandlers()
	a.context = []Message{{Role: "system", Content: "test content"}}
	if !a.appMain(mr) {
		t.Fatalf("appMain returned false")
	}
	p.expectNoOutput(t)
	p.expectNoWarnings(t)
	c.expectNoSentContent(t)
	if !strings.Contains(p.err.String(), "role") {
		t.Fatalf("expected error message to contain 'role'")
	}
	assertContextEquals(t, a.context, []Message{{Role: "system", Content: "test content"}})
}

func TestAppendCommandEmptyString(t *testing.T) {
	mr := &MockReadliner{lines: []string{"/append user"}}
	a, p, c := makeTestApp()
	a.registerCommandHandlers()
	a.context = []Message{{Role: "system", Content: "test content"}}
	if !a.appMain(mr) {
		t.Fatalf("appMain returned false")
	}
	p.expectNoOutput(t)
	p.expectNoErrors(t)
	c.expectNoSentContent(t)
	if !strings.Contains(p.warn.String(), "empty") {
		t.Fatalf("expected warning message to contain 'empty'")
	}
	assertContextEquals(t, a.context, []Message{{Role: "system", Content: "test content"}, {Role: "user", Content: ""}})
}

func TestAppendCommand(t *testing.T) {
	for _, role := range []string{"user", "system", "assistant"} {
		mr := &MockReadliner{lines: []string{"/append " + role + " abcdefgh"}}
		a, p, c := makeTestApp()
		a.registerCommandHandlers()
		a.context = []Message{{Role: "system", Content: "test content"}}
		if !a.appMain(mr) {
			t.Fatalf("appMain returned false")
		}
		p.expectNoOutput(t)
		p.expectNoErrors(t)
		p.expectNoWarnings(t)
		c.expectNoSentContent(t)
		assertContextEquals(t, a.context, []Message{{Role: "system", Content: "test content"}, {Role: role, Content: "abcdefgh"}})
	}
}

func TestPrependCommandNoArguments(t *testing.T) {
	assertCommandHasWrongNumberOfArguments(t, "/prepend")
}

func TestPrependCommandInvalidRole(t *testing.T) {
	mr := &MockReadliner{lines: []string{"/prepend invalid test message"}}
	a, p, c := makeTestApp()
	a.registerCommandHandlers()
	a.context = []Message{{Role: "system", Content: "test content"}}
	if !a.appMain(mr) {
		t.Fatalf("appMain returned false")
	}
	p.expectNoOutput(t)
	p.expectNoWarnings(t)
	c.expectNoSentContent(t)
	if !strings.Contains(p.err.String(), "role") {
		t.Fatalf("expected error message to contain 'role'")
	}
	assertContextEquals(t, a.context, []Message{{Role: "system", Content: "test content"}})
}

func TestPrependCommandEmptyString(t *testing.T) {
	mr := &MockReadliner{lines: []string{"/prepend user"}}
	a, p, c := makeTestApp()
	a.registerCommandHandlers()
	a.context = []Message{{Role: "system", Content: "test content"}}
	if !a.appMain(mr) {
		t.Fatalf("appMain returned false")
	}
	p.expectNoOutput(t)
	p.expectNoErrors(t)
	c.expectNoSentContent(t)
	if !strings.Contains(p.warn.String(), "empty") {
		t.Fatalf("expected warning message to contain 'empty'")
	}
	assertContextEquals(t, a.context, []Message{{Role: "user", Content: ""}, {Role: "system", Content: "test content"}})
}

func TestPrependCommand(t *testing.T) {
	for _, role := range []string{"user", "system", "assistant"} {
		mr := &MockReadliner{lines: []string{"/prepend " + role + " abcdefgh"}}
		a, p, c := makeTestApp()
		a.registerCommandHandlers()
		a.context = []Message{{Role: "system", Content: "test content"}}
		if !a.appMain(mr) {
			t.Fatalf("appMain returned false")
		}
		p.expectNoOutput(t)
		p.expectNoErrors(t)
		p.expectNoWarnings(t)
		c.expectNoSentContent(t)
		assertContextEquals(t, a.context, []Message{{Role: role, Content: "abcdefgh"}, {Role: "system", Content: "test content"}})
	}
}

func TestModelCommandNoArguments(t *testing.T) {
	assertCommandHasWrongNumberOfArguments(t, "/model")
}

func TestModelCommand(t *testing.T) {
	mr := &MockReadliner{lines: []string{"/model abcdef"}}
	a, p, c := makeTestApp()
	a.registerCommandHandlers()
	if !a.appMain(mr) {
		t.Fatalf("appMain returned false")
	}
	p.expectNoOutput(t)
	p.expectNoErrors(t)
	p.expectNoWarnings(t)
	c.expectNoSentContent(t)
	if *a.model != "abcdef" {
		t.Fatalf("app.model == %v, expect abcdef", *a.model)
	}
}

func TestPopCommandNonInteger(t *testing.T) {
	for _, value := range []string{"5.4", "abcabc", "///"} {
		mr := &MockReadliner{lines: []string{"/pop " + value}}
		a, p, c := makeTestApp()
		a.registerCommandHandlers()
		if !a.appMain(mr) {
			t.Fatalf("appMain returned false")
		}
		p.expectNoOutput(t)
		p.expectNoWarnings(t)
		c.expectNoSentContent(t)
		if p.err.Len() == 0 {
			t.Fatalf("expected error message")
		}
	}
}

func TestPopCommandNegativesZero(t *testing.T) {
	for _, value := range []string{"-10", "-1", "0"} {
		mr := &MockReadliner{lines: []string{"/pop " + value}}
		a, p, c := makeTestApp()
		a.registerCommandHandlers()
		if !a.appMain(mr) {
			t.Fatalf("appMain returned false")
		}
		p.expectNoOutput(t)
		p.expectNoWarnings(t)
		c.expectNoSentContent(t)
		if !strings.Contains(p.err.String(), "must be") {
			t.Fatalf("expected error message to contain 'must be'")
		}
	}
}

func TestPopCommandDefaultArgument(t *testing.T) {
	mr := &MockReadliner{lines: []string{"/pop"}}
	a, p, c := makeTestApp()
	a.registerCommandHandlers()
	a.context = []Message{
		{Role: "system", Content: "a"},
		{Role: "user", Content: "b"},
		{Role: "assistant", Content: "c"},
	}
	if !a.appMain(mr) {
		t.Fatalf("appMain returned false")
	}
	p.expectNoOutput(t)
	p.expectNoWarnings(t)
	p.expectNoErrors(t)
	c.expectNoSentContent(t)
	assertContextEquals(t, a.context, []Message{{Role: "system", Content: "a"}})
}

func TestPopCommand(t *testing.T) {
	for _, value := range []int{1, 2, 3} {
		expect := []Message{
			{Role: "system", Content: "a"},
			{Role: "user", Content: "b"},
			{Role: "assistant", Content: "c"},
		}
		mr := &MockReadliner{lines: []string{"/pop " + fmt.Sprint(value)}}
		a, p, c := makeTestApp()
		a.registerCommandHandlers()
		a.context = make([]Message, 0)
		a.context = append(a.context, expect...)
		if !a.appMain(mr) {
			t.Fatalf("appMain returned false")
		}
		p.expectNoOutput(t)
		p.expectNoWarnings(t)
		p.expectNoErrors(t)
		c.expectNoSentContent(t)
		assertContextEquals(t, a.context, expect[:len(expect)-value])
	}
}

func TestEscapeCommand(t *testing.T) {
	for _, content := range []string{"", "xyz", "/help"} {
		expect := []Message{
			{Role: "system", Content: "a"},
			{Role: "user", Content: "b"},
			{Role: "assistant", Content: "c"},
			{Role: "user", Content: content},
			{Role: "assistant", Content: "abc def"},
		}
		mr := &MockReadliner{lines: []string{"/escape " + content}}
		a, p, c := makeTestApp()
		a.registerCommandHandlers()
		a.context = make([]Message, 0)
		a.context = append(a.context, expect...)
		a.context = a.context[:len(a.context)-2]
		c.contentToSend = []CompletionDelta{{delta: "abc ", err: nil}, {delta: "def", err: nil}, {delta: "", err: io.EOF}}
		if !a.appMain(mr) {
			t.Fatalf("appMain returned false")
		}
		p.expectNoErrors(t)
		p.expectNoWarnings(t)
		if !strings.Contains(p.info.String(), "abc def") {
			t.Fatalf("expected output to contain 'abc def'")
		}
		assertContextEquals(t, c.receivedContext, expect[:len(expect)-1])
		assertContextEquals(t, a.context, expect)
	}
}

func TestSendCommandWithArguments(t *testing.T) {
	assertCommandHasWrongNumberOfArguments(t, "/send diwnixaw")
}

func TestSendCommand(t *testing.T) {
	expect := []Message{
		{Role: "system", Content: "a"},
		{Role: "user", Content: "b"},
		{Role: "assistant", Content: "c"},
		{Role: "user", Content: "d"},
		{Role: "assistant", Content: "abc def"},
	}
	mr := &MockReadliner{lines: []string{"/send"}}
	a, p, c := makeTestApp()
	a.registerCommandHandlers()
	a.context = make([]Message, 0)
	a.context = append(a.context, expect...)
	a.context = a.context[:len(a.context)-1]
	c.contentToSend = []CompletionDelta{{delta: "abc ", err: nil}, {delta: "def", err: nil}, {delta: "", err: io.EOF}}
	if !a.appMain(mr) {
		t.Fatalf("appMain returned false")
	}
	p.expectNoErrors(t)
	p.expectNoWarnings(t)
	if !strings.Contains(p.info.String(), "abc def") {
		t.Fatalf("expected output to contain 'abc def'")
	}
	assertContextEquals(t, c.receivedContext, expect[:len(expect)-1])
	assertContextEquals(t, a.context, expect)
}

func TestAutosaveAfterEveryMessage(t *testing.T) {
	expect := []Message{
		{Role: "system", Content: "a"},
		{Role: "user", Content: "b"},
		{Role: "assistant", Content: "abc def"},
		{Role: "user", Content: "c"},
		{Role: "assistant", Content: "abc def"},
		{Role: "user", Content: "d"},
		{Role: "assistant", Content: "abc def"},
	}
	lines := []string{"b", "c", "d"}
	autosavePath := temporaryFilePath()
	defer os.Remove(autosavePath)
	mr := &MockReadliner{lines: lines}
	a, p, c := makeTestApp()
	a.context = []Message{{Role: "system", Content: "a"}}
	a.autosaveFilePath = autosavePath
	c.contentToSend = []CompletionDelta{{delta: "abc ", err: nil}, {delta: "def", err: nil}, {delta: "", err: io.EOF}}
	a.registerCommandHandlers()
	for i := range lines {
		if !a.appMain(mr) {
			t.Fatalf("appMain returned false")
		}
		p.expectNoErrors(t)
		p.expectNoWarnings(t)
		if !strings.Contains(p.info.String(), "abc def") {
			t.Fatalf("expected output to contain 'abc def'")
		}
		assertContextEquals(t, c.receivedContext, expect[:2*i+2])
		assertContextEquals(t, a.context, expect[:2*i+3])
		saved, err := parseContextFile(autosavePath)
		if err != nil {
			panic(err)
		}
		assertContextEquals(t, saved, expect[:2*i+3])
		p.info.Reset()
		p.warn.Reset()
		p.err.Reset()
	}
}

func TestAutosaveCommand(t *testing.T) {
	file := temporaryFilePath()
	defer os.Remove(file)
	for _, value := range []string{"", file} {
		mr := &MockReadliner{lines: []string{"/autosave " + value}}
		a, p, c := makeTestApp()
		a.registerCommandHandlers()
		a.autosaveFilePath = "test"
		if !a.appMain(mr) {
			t.Fatalf("appMain returned false")
		}
		if a.autosaveFilePath != value {
			t.Fatalf("app.autoSaveFilePath: expect %v, got %v", value, a.autosaveFilePath)
		}
		p.expectNoOutput(t)
		p.expectNoWarnings(t)
		p.expectNoErrors(t)
		c.expectNoSentContent(t)
	}
}

func TestAutosaveCommandInvalidFile(t *testing.T) {
	path := "diwma/fnciauoc/cwaonjdcin.json"
	mr := &MockReadliner{lines: []string{"/autosave " + path}}
	a, p, c := makeTestApp()
	a.registerCommandHandlers()
	a.autosaveFilePath = "test"
	if !a.appMain(mr) {
		t.Fatalf("appMain returned false")
	}
	if a.autosaveFilePath != path {
		t.Fatalf("app.autoSaveFilePath: expect %v, got %v", path, a.autosaveFilePath)
	}
	p.expectNoOutput(t)
	p.expectNoWarnings(t)
	c.expectNoSentContent(t)
	if !strings.Contains(p.err.String(), "failed to write") {
		t.Fatalf("expected error to contain 'failed to write' but got %v", p.err.String())
	}
}

func TestAskQuestion(t *testing.T) {
	expect := []Message{
		{Role: "system", Content: "a"},
		{Role: "user", Content: "b"},
		{Role: "assistant", Content: "abc def"},
		{Role: "user", Content: "c"},
		{Role: "assistant", Content: "abc def"},
		{Role: "user", Content: "d"},
		{Role: "assistant", Content: "abc def"},
	}
	lines := []string{"b", "c", "d"}
	mr := &MockReadliner{lines: lines}
	a, p, c := makeTestApp()
	a.context = []Message{{Role: "system", Content: "a"}}
	c.contentToSend = []CompletionDelta{{delta: "abc ", err: nil}, {delta: "def", err: nil}, {delta: "", err: io.EOF}}
	a.registerCommandHandlers()
	for i := range lines {
		if !a.appMain(mr) {
			t.Fatalf("appMain returned false")
		}
		p.expectNoErrors(t)
		p.expectNoWarnings(t)
		if !strings.Contains(p.info.String(), "abc def") {
			t.Fatalf("expected output to contain 'abc def'")
		}
		assertContextEquals(t, c.receivedContext, expect[:2*i+2])
		assertContextEquals(t, a.context, expect[:2*i+3])
		p.info.Reset()
		p.warn.Reset()
		p.err.Reset()
	}
}
