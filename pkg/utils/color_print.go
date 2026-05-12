package utils

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
)

var (
	silent  = false
	logFile io.WriteCloser
	logOnce sync.Once
)

const (
	blue   = "\033[34m"
	green  = "\033[32m"
	yellow = "\033[33m"
	red    = "\033[31m"
	reset  = "\033[0m"

	Logo = blue +
		"▗▖  ▗▖▗▄▄▄▖ ▗▄▄▖ ▗▄▖\n" +
		"▐▛▚▞▜▌  █  ▐▌   ▐▌ ▐▌\n" +
		"▐▌  ▐▌  █  ▐▌   ▐▌ ▐▌\n" +
		"▐▌  ▐▌▗▄█▄▖▝▚▄▄▖▝▚▄▞▘\n" +
		reset
)

func initLog() {
	logOnce.Do(func() {
		path := GetLogPath()
		if path == "" {
			return
		}
		os.MkdirAll(filepath.Dir(path), 0755)
		f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err == nil {
			logFile = f
		}
	})
}

func logWrite(f string, a ...any) {
	initLog()
	if logFile != nil {
		fmt.Fprintf(logFile, f, a...)
	}
}

func PrintI(f string, a ...any) {
	if silent {
		logWrite(f, a...)
	} else {
		fmt.Printf(blue+f+reset, a...)
	}
}

func PrintS(f string, a ...any) {
	if silent {
		logWrite(f, a...)
	} else {
		fmt.Printf(green+f+reset, a...)
	}
}

func PrintW(f string, a ...any) {
	if silent {
		logWrite(f, a...)
	} else {
		fmt.Printf(yellow+f+reset, a...)
	}
}

func PrintE(f string, a ...any) {
	if silent {
		logWrite(f, a...)
	} else {
		fmt.Printf(red+f+reset, a...)
	}
}

func SetSilent() {
	silent = true
}
