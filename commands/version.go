package commands

import "fmt"

var Version = "dev-build"

func PrintVersion() {
	fmt.Println("Version", Version)
}
