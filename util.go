package main

import (
	"log"
	"strings"
)

// PaddingLeft is a fmt.printxx utility
func PaddingLeft(original string, maxLen int, char string) string {
	if n := maxLen - len(original); n > 0 {
		return strings.Repeat(char, n) + original
	}
	return original
}

// Logln is a log.Println wrapper that only writes to log when the verbose flag is set
func Logln(v ...interface{}) {
	if verbose {
		log.Println(v...)
	}
}

// Logf is a log.Printf wrapper that only writes to log when the verbose flag is set
func Logf(format string, args ...interface{}) {
	if verbose {
		log.Printf(format, args...)
	}
}
