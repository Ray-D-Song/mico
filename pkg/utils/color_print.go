package utils

import "fmt"

var slient = false

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

func PrintI(f string, a ...any) {
	if !slient {
		fmt.Printf(blue+f+reset, a...)
	}
}

func PrintS(f string, a ...any) {
	if !slient {
		fmt.Printf(green+f+reset, a...)
	}
}

func PrintW(f string, a ...any) {
	if !slient {
		fmt.Printf(yellow+f+reset, a...)
	}
}

func PrintE(f string, a ...any) {
	if !slient {
		fmt.Printf(red+f+reset, a...)
	}
}

func SetSlient() {
	slient = true
}
