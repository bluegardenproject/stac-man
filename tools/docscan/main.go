// docscan dumps the stac-man cobra command tree as JSON. It's the
// programmatic source for the docs-drift-audit Cursor skill, which
// cross-checks the binary surface against the docs/site/ pages.
//
// Usage:
//
//	go run ./tools/docscan
//	go run ./tools/docscan | jq '.commands[].name'
//
// Output is a stable shape: tweak it cautiously, the skill parses it.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/bluegardenproject/stac-man/cmd"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type flagInfo struct {
	Name      string `json:"name"`
	Shorthand string `json:"shorthand,omitempty"`
	Usage     string `json:"usage"`
	Default   string `json:"default,omitempty"`
	Type      string `json:"type"`
}

type commandInfo struct {
	Name        string     `json:"name"`
	Use         string     `json:"use"`
	Short       string     `json:"short"`
	Long        string     `json:"long,omitempty"`
	Aliases     []string   `json:"aliases,omitempty"`
	Path        string     `json:"path"` // full command path, e.g. "sm checkout"
	Hidden      bool       `json:"hidden,omitempty"`
	HasArgs     bool       `json:"hasArgs"`
	Flags       []flagInfo `json:"flags,omitempty"`
	Subcommands []string   `json:"subcommands,omitempty"`
}

type tree struct {
	Binary    string        `json:"binary"`
	Commands  []commandInfo `json:"commands"`
	GenSchema string        `json:"genSchema"`
}

func main() {
	root := cmd.Root()
	out := tree{
		Binary:    root.Use,
		GenSchema: "v1",
	}

	var walk func(c *cobra.Command, path string)
	walk = func(c *cobra.Command, path string) {
		fullPath := path
		if path == "" {
			fullPath = c.Use
		} else {
			fullPath = path + " " + nameOnly(c.Use)
		}

		info := commandInfo{
			Name:    nameOnly(c.Use),
			Use:     c.Use,
			Short:   c.Short,
			Long:    c.Long,
			Aliases: c.Aliases,
			Path:    fullPath,
			Hidden:  c.Hidden,
			HasArgs: c.Args != nil,
		}

		c.LocalFlags().VisitAll(func(f *pflag.Flag) {
			info.Flags = append(info.Flags, flagInfo{
				Name:      f.Name,
				Shorthand: f.Shorthand,
				Usage:     f.Usage,
				Default:   f.DefValue,
				Type:      f.Value.Type(),
			})
		})
		sort.Slice(info.Flags, func(i, j int) bool { return info.Flags[i].Name < info.Flags[j].Name })

		for _, sub := range c.Commands() {
			info.Subcommands = append(info.Subcommands, nameOnly(sub.Use))
		}
		sort.Strings(info.Subcommands)

		out.Commands = append(out.Commands, info)

		for _, sub := range c.Commands() {
			walk(sub, fullPath)
		}
	}
	walk(root, "")

	sort.Slice(out.Commands, func(i, j int) bool { return out.Commands[i].Path < out.Commands[j].Path })

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// nameOnly returns the first whitespace-separated token of a cobra
// command's Use string. cobra encodes args/synopsis after the name
// (e.g. "checkout [branch]"), but for the JSON we want just "checkout".
func nameOnly(use string) string {
	for i, r := range use {
		if r == ' ' || r == '\t' {
			return use[:i]
		}
	}
	return use
}
