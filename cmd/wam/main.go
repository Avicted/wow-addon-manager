// Command wam keeps WoW Forever addons up to date from CurseForge and GitHub.
package main

import (
	"os"

	"github.com/Avicted/wow-addon-manager/internal/cli"
)

// version is set at build time: -ldflags "-X main.version=$(cat VERSION)".
var version = "dev"

func main() { os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr, version)) }
