package utils

import "fmt"

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
	fmt.Printf(blue+f+reset, a...)
}

func PrintS(f string, a ...any) {
	fmt.Printf(green+f+reset, a...)
}

func PrintW(f string, a ...any) {
	fmt.Printf(yellow+f+reset, a...)
}

func PrintE(f string, a ...any) {
	fmt.Printf(red+f+reset, a...)
}
