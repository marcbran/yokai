package terminal

import (
	"fmt"
	"github.com/charmbracelet/lipgloss"
	"os"
)

var info = lipgloss.NewStyle().Foreground(lipgloss.Color("4"))
var warn = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
var fail = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
var success = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))

func Space() {
	fmt.Fprintln(os.Stderr, "")
}

func Info(text string) {
	_, _ = fmt.Fprintln(os.Stderr, info.Render(text))
}

func Infof(text string, a ...any) {
	Info(fmt.Sprintf(text, a...))
}

func Warn(text string) {
	_, _ = fmt.Fprintln(os.Stderr, warn.Render(text))
}

func Warnf(text string, a ...any) {
	Warn(fmt.Sprintf(text, a...))
}

func Fail(text string) {
	_, _ = fmt.Fprintln(os.Stderr, fail.Render(text))
}

func Failf(text string, a ...any) {
	Fail(fmt.Sprintf(text, a...))
}

func Success(text string) {
	_, _ = fmt.Fprintln(os.Stderr, success.Render(text))
}

func Successf(text string, a ...any) {
	Success(fmt.Sprintf(text, a...))
}
