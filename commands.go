package main

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/fatih/color"
)

type Command struct {
	fn          func(*App, string) error
	description string
	args        [][]string
}

var ErrExpectNoArguments = fmt.Errorf("expected no arguments")

func (app *App) registerCommandHandlers() {
	app.commandHandlers = map[string]Command{
		"help":        NewCommand(helpCommand, `Shows this help page.`, [][]string{}),
		"save":        NewCommand(saveCommand, `Saves current conversation context in a JSON file.`, [][]string{{"path"}}),
		"replacefrom": NewCommand(replaceFromCommand, `Replaces the current conversation context from JSON file in the same format as created by /save.`, [][]string{{"path"}}),
		"appendfrom":  NewCommand(appendFromCommand, `Appends the context from the JSON file to the current context.`, [][]string{{"path"}}),
		"prependfrom": NewCommand(prependFromCommand, `Adds the context from the JSON file to the beggining of the current context.`, [][]string{{"path"}}),
		"clear":       NewCommand(clearCommand, `Clears the current conversation context.`, [][]string{}),
		"print":       NewCommand(printCommand, `Prints the current conversation context.`, [][]string{}),
		"append":      NewCommand(appendCommand, `Appends a message to the current conversation context.`, [][]string{{"user", "assistant", "system"}, {"message"}}),
		"prepend":     NewCommand(prependCommand, `Adds a message to the beggining of the current conversation context.`, [][]string{{"user", "assistant", "system"}, {"message"}}),
		"model":       NewCommand(modelCommand, `Switches the current model (e.g. gpt-3.5-turbo), keeping the conversation context.`, [][]string{{"model-name"}}),
		"pop": NewCommand(popCommand, `Removes the last N messages from the context. N defaults to 2, as to pop the last answer given by the model and
		the question that led to it.`, [][]string{{"N?"}}),
		"escape": NewCommand(escapeCommand, `Appends the following text and sends the context to the model, storing its response in the context. Useful for
		sending empty strings or messages beggining with the slash "/" character.`, [][]string{{"text"}}),
		"nano": NewCommand(nanoCommand, `Opens a nano (by default) text editor instance. You can write a multi-line prompt in it, which will be appended
		to the context (without sending it) once saved and closed. To use a different text editor, specify its path in the GPTREPL_TEXT_EDITOR environment variable.
		See also /ns, which may be more useful for interactive sessions in most cases.`, [][]string{{"user", "assistant", "system"}}),
		"ns":   NewCommand(nanoSendCommand, `The same as running /nano and then /send. Role is set to "user" by default. Also prints the message when not in quiet mode.`, [][]string{{"user?", "assistant?", "system?"}}),
		"send": NewCommand(sendCommand, `Sends the current context as-is to the model and stores its response in the context.`, [][]string{}),
		"autosave": NewCommand(autosaveCommand, `Changes the autosave file path. Every time the context changes, it is automatically saved to this file. Run
		with no arguments to disable this feature. WARNING: The file will be overwritten. You may want to load it first with
		/replacefrom, /appendfrom or /prependfrom.`, [][]string{{"path?"}}),
		"exit": NewCommand(exitCommand, `Exits the program.`, [][]string{{"status-code?"}}),
	}
}

func NewCommand(fn func(*App, string) error, description string, args [][]string) Command {
	return Command{fn, description, args}
}

func helpCommand(app *App, args string) error {
	if args != "" {
		app.printer.PrintWarning("This command takes no arguments. Showing help anyways\n")
	}
	for name, command := range app.commandHandlers {
		app.printer.Print("/%v", color.CyanString(name))
		for _, choices := range command.args {
			colored := make([]string, len(choices))
			for i, choice := range choices {
				colored[i] = color.MagentaString(choice)
			}
			app.printer.Print(" <%v>", strings.Join(colored, "|"))
		}
		app.printer.Print("\n")
		for _, line := range textWrap(command.description, 50) {
			app.printer.Print("  %v\n", line)
		}
	}
	return nil
}

func saveCommand(app *App, path string) error {
	if path == "" {
		return fmt.Errorf("exactly one argument required (path to JSON file)")
	}
	return writeContextFile(path, app.context)
}

func replaceFromCommand(app *App, path string) error {
	ctx, err := readContextFileFromArguments(path)
	if err != nil {
		return err
	}
	app.context = ctx
	return nil
}

func appendFromCommand(app *App, path string) error {
	ctx, err := readContextFileFromArguments(path)
	if err != nil {
		return err
	}
	app.context = append(app.context, ctx...)
	return nil
}

func prependFromCommand(app *App, path string) error {
	ctx, err := readContextFileFromArguments(path)
	if err != nil {
		return err
	}
	ctx = append(ctx, app.context...)
	app.context = ctx
	return nil
}

func clearCommand(app *App, args string) error {
	if args != "" {
		return ErrExpectNoArguments
	}
	app.context = make([]Message, 0)
	app.tryUpdateAutosaveFile()
	return nil
}

func printCommand(app *App, args string) error {
	if args != "" {
		app.printer.PrintWarning("this command takes no arguments. Printing context anyways\n")
	}
	c := color.Set(color.Bold, color.FgWhite)
	for _, msg := range app.context {
		app.printer.Print("%v%v%v\n", color.CyanString("["), c.Sprintf("%v", msg.Role), color.CyanString("]"))
		app.printer.Print("%v\n\n", msg.Content)
	}
	return nil
}

func appendCommand(app *App, args string) error {
	role, msg, err := parseSingleMessageFromArguments(args)
	if err != nil {
		return err
	}
	msg = strings.TrimSpace(msg)
	if msg == "" {
		app.printer.PrintWarning("appending empty string to context\n")
	}
	app.appendToContext(Message{Role: role, Content: msg})
	return nil
}

func prependCommand(app *App, args string) error {
	role, msg, err := parseSingleMessageFromArguments(args)
	if err != nil {
		return err
	}
	msg = strings.TrimSpace(msg)
	if msg == "" {
		app.printer.PrintWarning("prepending empty string to context\n")
	}
	new := make([]Message, 0, 1+len(app.context))
	new = append(new, Message{Role: role, Content: msg})
	new = append(new, app.context...)
	app.context = new
	return nil
}

func modelCommand(app *App, model string) error {
	if model == "" {
		return fmt.Errorf("expected exactly one argument (the identifier of the model)")
	}
	*app.model = model
	return nil
}

func popCommand(app *App, args string) error {
	n, err := parseSingleIntegerFromArguments(args, 2)
	if err != nil {
		return err
	}
	return app.popFromContext(int(n))
}

func escapeCommand(app *App, messageContent string) error {
	app.appendToContext(Message{Role: "user", Content: messageContent})
	resp, err := app.sendContextAndProcessResponse()
	if err != nil {
		return err
	}
	app.appendToContext(Message{Role: "assistant", Content: resp})
	return nil
}

func nanoCommand(app *App, role string) error {

	if !isRoleValid(role) {
		return fmt.Errorf("invalid role: \"%v\"", role)
	}

	content, err := presentTextEditor()
	if err != nil {
		return err
	}

	content = strings.TrimSpace(content)
	if content == "" {
		return fmt.Errorf("no content in file")
	}

	app.appendToContext(Message{Role: role, Content: content})
	return nil
}

func sendCommand(app *App, args string) error {
	if args != "" {
		return ErrExpectNoArguments
	}
	responseContent, err := app.sendContextAndProcessResponse()
	if err != nil {
		return err
	}
	app.appendToContext(Message{Role: "assistant", Content: responseContent})
	return nil
}

func nanoSendCommand(app *App, role string) error {
	if role == "" {
		role = "user"
	}
	if !isRoleValid(role) {
		return fmt.Errorf("invalid role: \"%v\"", role)
	}

	content, err := presentTextEditor()
	if err != nil {
		return err
	}

	content = strings.TrimSpace(content)
	if content == "" {
		return fmt.Errorf("no content in file")
	}

	app.appendToContext(Message{Role: role, Content: content})
	app.printer.Print("%v\n", content)

	responseContent, err := app.sendContextAndProcessResponse()
	if err != nil {
		return err
	}
	app.appendToContext(Message{Role: "assistant", Content: responseContent})

	return nil
}

func autosaveCommand(app *App, path string) error {
	app.autosaveFilePath = path
	app.tryUpdateAutosaveFile()
	return nil
}

func exitCommand(app *App, code string) error {
	n, err := parseSingleIntegerFromArguments(code, 0)
	if err != nil {
		return err
	}
	os.Exit(int(n))
	return nil
}

func parseSingleIntegerFromArguments(args string, defaultValue int) (int, error) {
	var n int64
	var err error
	if args == "" {
		n = int64(defaultValue)
	} else {
		n, err = strconv.ParseInt(args, 10, 32)
		if err != nil {
			return 0, err
		}
	}
	return int(n), nil
}

func parseSingleMessageFromArguments(args string) (string, string, error) {
	if args == "" {
		return "", "", fmt.Errorf("expected two arguments")
	}
	role, msg, _ := strings.Cut(args, " ")
	if !isRoleValid(role) {
		return "", "", fmt.Errorf("invalid role: \"%v\"", role)
	}
	return role, msg, nil
}

func readContextFileFromArguments(args string) ([]Message, error) {
	if args == "" {
		return nil, fmt.Errorf("exactly one argument required (path to JSON file)")
	}
	ctx, err := parseContextFile(args)
	if err != nil {
		return nil, err
	}
	return ctx, nil
}

func presentTextEditor() (string, error) {
	temp, err := os.CreateTemp("", "gptrepl")
	if err != nil {
		return "", fmt.Errorf("failed to create temporary file: %v", err)
	}
	temp.Close()
	defer os.Remove(temp.Name())
	os.Chmod(temp.Name(), 0777)
	editorPath := os.Getenv("GPTREPL_TEXT_EDITOR")
	if editorPath == "" {
		editorPath = "nano"
	}

	cmd := exec.Command(editorPath, temp.Name())
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr

	err = cmd.Run()
	if err != nil {
		return "", fmt.Errorf("text editor failed to run: %v", err)
	}
	content, err := os.ReadFile(temp.Name())
	if err != nil {
		return "", fmt.Errorf("can't read temporary file: %v", err)
	}
	return string(content), nil
}
