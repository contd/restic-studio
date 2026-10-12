package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/restic-studio/restic-studio/internal/config"
	"github.com/restic-studio/restic-studio/internal/tui"
)

func main() {
	defaultConfig, err := config.DefaultPath()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	configPath := flag.String("config", defaultConfig, "path to an Autorestic v2 YAML configuration")
	flag.Parse()

	absolutePath, err := tui.ResolveConfigPath(*configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "resolve config path:", err)
		os.Exit(1)
	}
	cfg, err := config.Load(absolutePath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := tui.InitProgram(cfg); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
