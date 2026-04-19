package utils

import "fmt"

const (
	blue   = "\033[34m"
	green  = "\033[32m"
	yellow = "\033[33m"
	red    = "\033[31m"
	reset  = "\033[0m"
)

func printI(f string, a ...any) {
	fmt.Printf(blue+f+reset, a...)
}

func printS(f string, a ...any) {
	fmt.Printf(green+f+reset, a...)
}

func printW(f string, a ...any) {
	fmt.Printf(yellow+f+reset, a...)
}

func printE(f string, a ...any) {
	fmt.Printf(red+f+reset, a...)
}

