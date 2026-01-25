package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("tapedeck-cli: no command specified")
		os.Exit(1)
	}
	fmt.Printf("tapedeck-cli: %s\n", os.Args[1])
}
