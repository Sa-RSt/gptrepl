package main

import (
	"fmt"
	"os"

	"github.com/fatih/color"
)

type UserPrinter interface {
	Print(string, ...interface{})
	PrintWarning(string, ...interface{})
	PrintError(string, ...interface{})
}

type ConsoleUserPrinter struct{}

func (*ConsoleUserPrinter) Print(format string, a ...interface{}) {
	fmt.Printf(format, a...)
	os.Stdout.Sync()
}

func (*ConsoleUserPrinter) PrintWarning(format string, a ...interface{}) {
	fmt.Fprint(os.Stderr, color.YellowString("Warning: "), fmt.Sprintf(format, a...))
	os.Stderr.Sync()
}

func (*ConsoleUserPrinter) PrintError(format string, a ...interface{}) {
	fmt.Fprint(os.Stderr, color.RedString("Error: "), fmt.Sprintf(format, a...))
	os.Stderr.Sync()
}
