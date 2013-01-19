package main

import (
	"fmt"
	"log"
)

const (
	// terminal reset
	TR         = "\x1b[0m"
	Bright     = "\x1b[1m"
	Dim        = "\x1b[2m"
	Underscore = "\x1b[4m"
	Blink      = "\x1b[5m"
	Reverse    = "\x1b[7m"
	Hidden     = "\x1b[8m"

	FgBlack   = "\x1b[30m"
	FgRed     = "\x1b[31m"
	FgGreen   = "\x1b[32m"
	FgYellow  = "\x1b[33m"
	FgBlue    = "\x1b[34m"
	FgMagenta = "\x1b[35m"
	FgCyan    = "\x1b[36m"
	FgWhite   = "\x1b[37m"

	BgBlack   = "\x1b[40m"
	BgRed     = "\x1b[41m"
	BgGreen   = "\x1b[42m"
	BgYellow  = "\x1b[43m"
	BgBlue    = "\x1b[44m"
	BgMagenta = "\x1b[45m"
	BgCyan    = "\x1b[46m"
	BgWhite   = "\x1b[47m"
)

func Talk(v ...interface{}) {
	if !*verbose {
		return
	}

	Log(FgCyan, "🍃 ", v...)
}

func Note(v ...interface{}) {
	Log(FgGreen, "✏ ", v...)
}

func Warn(v ...interface{}) {
	Log(FgYellow, "📢 ", v...)
}

func Err(v ...interface{}) {
	Log(FgRed, "❗ ", v...)
}

func Fatal(v ...interface{}) {
	Log(FgRed+Bright, "‼ ",v...)
}

func Log(color string, icon string, v ...interface{}) {
	v[0] = fmt.Sprintf("%v%v%v%v", color, icon, v[0], TR)

	log.Print(v...)
}
