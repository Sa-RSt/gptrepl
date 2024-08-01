package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path"
	"strings"

	"github.com/chzyer/readline"
	"github.com/fatih/color"
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type App struct {
	context               []Message
	model                 *string
	slashCommandsDisabled bool
	quiet                 bool
	apiKey                *string
	commandHandlers       map[string]Command
	autosaveFilePath      string
	printer               UserPrinter
	capi                  CompletionAPI
}

type CompletionDelta struct {
	delta string
	err   error
}

func main() {
	var app App
	app.configure()
	app.registerCommandHandlers()
	app.mainLoop()
}

func (app *App) configure() {
	app.printer = &ConsoleUserPrinter{}
	app.parseFlags()
	if !app.fillApiKeyIfNotPresent() {
		printApiKeyHelpMessage(app.printer)
		os.Exit(1)
	}
	app.capi = &OpenAICompletionAPI{apiKey: app.apiKey, model: app.model}
}

func (app *App) mainLoop() {
	if !app.quiet && !app.slashCommandsDisabled {
		app.printer.Print("Enter \"%v\" for a list of commands.\n", color.GreenString("/help"))
	}
	var prompt string
	if app.quiet {
		prompt = ""
	} else {
		prompt = fmt.Sprintf("%v%v%v%v", color.BlueString("("), color.YellowString(*app.model), color.BlueString(")"), color.CyanString("> "))
	}
	reader, err := readline.New(prompt)
	if err != nil {
		app.printer.PrintError("failed to initialize readline: %v\n", err)
		return
	}
	defer reader.Close()
	for app.appMain(reader) {

	}
}

type Readliner interface {
	Readline() (string, error)
}

func (app *App) appMain(reader Readliner) bool {
	line, err := reader.Readline()
	if err != nil {
		return false
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return true
	}
	if line[0] == '/' && !app.slashCommandsDisabled {
		commandName, arguments, _ := strings.Cut(line, " ")
		commandName = strings.TrimSpace(commandName[1:])
		command, ok := app.commandHandlers[commandName]
		if !ok {
			app.printer.PrintError("unknown command: %v\n", commandName)
			return true
		}
		err = command.fn(app, strings.TrimSpace(arguments))
		if err != nil {
			app.printer.PrintError("%v: %v\n", commandName, err)
		}
		return true
	}
	app.appendToContext(Message{Role: "user", Content: line})
	responseContent, err := app.sendContextAndProcessResponse()
	if err != nil {
		app.printer.PrintError("%v (no changes done to context)\n", err)
		app.popFromContext(1)
		return true
	}
	app.appendToContext(Message{Role: "assistant", Content: responseContent})
	return true
}

func (app *App) sendContextAndProcessResponse() (string, error) {
	stream, err := app.capi.SendContext(app.context)
	if err != nil {
		return "", fmt.Errorf("failed to send context: %v", err)
	}
	responseContent, err := printAndCollectStream(app.printer, stream)
	if err != nil {
		return "", fmt.Errorf("stream error: %v", err)
	}
	return responseContent, nil
}

func printAndCollectStream(printer UserPrinter, stream <-chan CompletionDelta) (string, error) {
	var collect bytes.Buffer
	for {
		response, ok := <-stream
		if !ok || errors.Is(response.err, io.EOF) {
			printer.Print("\n")
			return collect.String(), nil
		}

		if response.err != nil {
			return "", response.err
		}

		collect.WriteString(response.delta)
		printer.Print("%v", response.delta)
	}
}

func (app *App) appendToContext(msg Message) {
	app.context = append(app.context, msg)
	app.tryUpdateAutosaveFile()
}

func (app *App) popFromContext(n int) error {
	if n < 1 {
		return fmt.Errorf("n must be at least 1")
	}
	if n > len(app.context) {
		return fmt.Errorf("can't pop %v elements from the context because it only contains %v elements", n, len(app.context))
	}
	app.context = app.context[:len(app.context)-n]
	app.tryUpdateAutosaveFile()
	return nil
}

func (app *App) tryUpdateAutosaveFile() {
	if app.autosaveFilePath == "" {
		return
	}
	err := writeContextFile(app.autosaveFilePath, app.context)
	if err != nil {
		app.printer.PrintError("failed to write to file \"%v\": %v\n", app.autosaveFilePath, err)
	}
}

func (app *App) parseFlags() {
	addJsonCtx := func(path string) error {
		messages, err := parseContextFile(path)
		if err != nil {
			return err
		}
		app.context = append(app.context, messages...)
		return nil
	}
	model := ""
	apiKey := ""
	app.model = &model
	app.apiKey = &apiKey
	flag.Func("ctx", "Load and append a JSON context file (such as one created by the /save interactive command). Can be used multiple times.", addJsonCtx)
	flag.StringVar(app.model, "model", "gpt-4", "The OpenAI model ID string (e.g. gpt-3.5-turbo).")
	flag.StringVar(app.apiKey, "apikey", "", "The OpenAI API key to use. Overrides $OPENAI_API_KEY and ~/.gptrepl-key.")
	flag.BoolVar(&app.slashCommandsDisabled, "nocommands", false, "Disable slash (\"/\") commands.")
	flag.BoolVar(&app.quiet, "quiet", false, "Only print the model's output (errors will still be printed to stderr).")
	flag.StringVar(&app.autosaveFilePath, "autosave", "", `Load the path as a JSON context (if it exists) and sets it as the autosave file path. The context is automatically saved to this file after every update. This file is always the last one loaded, regardless of its ordering relative to the -ctx flags.`)
	autosavePreventLoad := flag.Bool("autosave-prevent-load", false, "Prevent the file specified in the -autosave flag from being loaded. Ignored if -autosave isn't set.")
	flag.Parse()

	_, err := os.Stat(app.autosaveFilePath)
	if !*autosavePreventLoad && app.autosaveFilePath != "" && !errors.Is(err, os.ErrNotExist) {
		err = addJsonCtx(app.autosaveFilePath)
		if err != nil {
			app.printer.PrintError("failed to load autosave file: %v", err)
			os.Exit(1)
		}
	}
}

func (app *App) fillApiKeyIfNotPresent() bool {
	if *app.apiKey != "" {
		return true
	}

	key := os.Getenv("OPENAI_API_KEY")
	if key != "" {
		*app.apiKey = key
		return true
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	keyBytes, err := os.ReadFile(path.Join(home, ".gptrepl-key"))
	if err != nil {
		return false
	}
	*app.apiKey = strings.TrimSpace(string(keyBytes))
	return true
}

func printApiKeyHelpMessage(printer UserPrinter) {
	home, err := os.UserHomeDir()
	var comp string
	if err == nil {
		comp = fmt.Sprintf("(currently %v) ", home)
	} else {
		comp = ""
	}

	printer.PrintError("An %v was not provided.\n", color.RedString("OpenAI API key"))
	printer.PrintError("gptrepl searches for the key in three places until one is found, in the following order:\n")
	printer.PrintError(" - The -apikey command-line flag\n")
	printer.PrintError(" - OPENAI_API_KEY environment variable\n")
	printer.PrintError(" - A file named \".gptrepl-key\" located in the home directory %vcontaining only a plaintext key in UTF-8 encoding.\n", comp)
}
