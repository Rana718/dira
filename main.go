package main

import "github.com/Rana718/dira/cmd"

var version = "0.1.0"

func main() {
	cmd.SetVersion(version)
	cmd.Execute()
}
