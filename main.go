// An app configured in Nickel: it evaluates config.ncl (or the file given as
// its first argument), whose contracts validate it, into a Go struct, and
// prints the greetings it would serve.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/nickel-lang/go-nickel"
)

// Config mirrors schema.ncl; ConvertTo fills it through the json tags
type Config struct {
	Name      string            `json:"name"`
	Port      int               `json:"port"`
	Greetings map[string]string `json:"greetings"`
}

func loadConfig(path string) (*Config, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// go-nickel evaluates source text, and Nickel looks for imports in the
	// current directory, so evaluate from the config file's own directory
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	if err := os.Chdir(filepath.Dir(path)); err != nil {
		return nil, err
	}
	defer func() { _ = os.Chdir(wd) }()

	expr, err := nickel.NewContext().EvalDeep(string(src))
	if err != nil {
		return nil, err
	}
	var config Config
	if err := expr.ConvertTo(&config); err != nil {
		return nil, err
	}
	return &config, nil
}

func main() {
	path := "config.ncl"
	if len(os.Args) > 1 {
		path = os.Args[1]
	}

	config, err := loadConfig(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	fmt.Printf("%s would listen on port %d and serve:\n", config.Name, config.Port)
	codes := make([]string, 0, len(config.Greetings))
	for code := range config.Greetings {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	for _, code := range codes {
		fmt.Printf("  /%s: %s\n", code, config.Greetings[code])
	}
}
