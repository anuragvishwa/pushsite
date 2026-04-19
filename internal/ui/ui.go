package ui

import (
	"fmt"
	"os"
	"time"

	"github.com/fatih/color"
	"github.com/schollz/progressbar/v3"
)

type UI struct {
	verbose bool
}

func New(verbose bool) *UI {
	return &UI{verbose: verbose}
}

var (
	successColor = color.New(color.FgGreen, color.Bold)
	errorColor   = color.New(color.FgRed, color.Bold)
	warnColor    = color.New(color.FgYellow)
	infoColor    = color.New(color.FgCyan)
	dimColor     = color.New(color.Faint)
	boldColor    = color.New(color.Bold)
)

func (u *UI) Success(format string, args ...interface{}) {
	successColor.Print("✓ ")
	fmt.Printf(format+"\n", args...)
}

func (u *UI) Error(format string, args ...interface{}) {
	errorColor.Print("✗ ")
	fmt.Fprintf(os.Stderr, format+"\n", args...)
}

func (u *UI) Warn(format string, args ...interface{}) {
	warnColor.Print("! ")
	fmt.Printf(format+"\n", args...)
}

func (u *UI) Info(format string, args ...interface{}) {
	infoColor.Print("→ ")
	fmt.Printf(format+"\n", args...)
}

func (u *UI) Debug(format string, args ...interface{}) {
	if u.verbose {
		dimColor.Printf("  %s\n", fmt.Sprintf(format, args...))
	}
}

func (u *UI) Step(step int, total int, format string, args ...interface{}) {
	dimColor.Printf("[%d/%d] ", step, total)
	fmt.Printf(format+"\n", args...)
}

func (u *UI) Title(format string, args ...interface{}) {
	fmt.Println()
	boldColor.Printf(format+"\n", args...)
	fmt.Println()
}

func (u *UI) Print(format string, args ...interface{}) {
	fmt.Printf(format+"\n", args...)
}

func (u *UI) NewLine() {
	fmt.Println()
}

type Spinner struct {
	bar     *progressbar.ProgressBar
	message string
	done    chan bool
}

func (u *UI) Spinner(message string) *Spinner {
	bar := progressbar.NewOptions(-1,
		progressbar.OptionSetDescription(message),
		progressbar.OptionSetWriter(os.Stderr),
		progressbar.OptionSpinnerType(14),
		progressbar.OptionSetRenderBlankState(true),
	)

	s := &Spinner{
		bar:     bar,
		message: message,
		done:    make(chan bool),
	}

	go func() {
		for {
			select {
			case <-s.done:
				return
			default:
				bar.Add(1)
				time.Sleep(100 * time.Millisecond)
			}
		}
	}()

	return s
}

func (s *Spinner) Stop() {
	s.done <- true
	s.bar.Finish()
	fmt.Println()
}

func (s *Spinner) Success(message string) {
	s.done <- true
	s.bar.Finish()
	fmt.Println()
	successColor.Print("✓ ")
	fmt.Println(message)
}

func (s *Spinner) Error(message string) {
	s.done <- true
	s.bar.Finish()
	fmt.Println()
	errorColor.Print("✗ ")
	fmt.Fprintln(os.Stderr, message)
}

func (u *UI) ProgressBar(total int, description string) *progressbar.ProgressBar {
	return progressbar.NewOptions(total,
		progressbar.OptionSetDescription(description),
		progressbar.OptionSetWriter(os.Stderr),
		progressbar.OptionShowBytes(true),
		progressbar.OptionSetWidth(40),
		progressbar.OptionThrottle(65*time.Millisecond),
		progressbar.OptionShowCount(),
		progressbar.OptionOnCompletion(func() {
			fmt.Println()
		}),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "=",
			SaucerHead:    ">",
			SaucerPadding: " ",
			BarStart:      "[",
			BarEnd:        "]",
		}),
	)
}
