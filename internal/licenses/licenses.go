// Copyright 2026 Yannick Müller
// SPDX-License-Identifier: Apache-2.0

// Package licenses carries the license texts of kubeswitch and of the
// libraries compiled into it, for the "license" command.
package licenses

import (
	"embed"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"
)

// Project is the license kubeswitch is distributed under and Notice its
// copyright notice: copies of the files in the repository root, kept in
// sync by "make licenses".
//
//go:embed LICENSE
var Project string

//go:embed NOTICE
var Notice string

// thirdParty holds a directory per dependency, named after its module
// path, with the license files shipped in the module. "make licenses"
// regenerates it from the Go module cache.
//
//go:embed third_party
var thirdParty embed.FS

// Dependency is a library kubeswitch is built with.
type Dependency struct {
	Module string            // module path, e.g. github.com/spf13/cobra
	Files  map[string]string // license file name -> contents
}

// Dependencies lists the third-party libraries with their license texts,
// sorted by module path.
func Dependencies() ([]Dependency, error) {
	byModule := map[string]*Dependency{}
	err := fs.WalkDir(thirdParty, "third_party", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := fs.ReadFile(thirdParty, p)
		if err != nil {
			return err
		}
		module, file := path.Split(strings.TrimPrefix(p, "third_party/"))
		module = strings.TrimSuffix(module, "/")
		dep := byModule[module]
		if dep == nil {
			dep = &Dependency{Module: module, Files: map[string]string{}}
			byModule[module] = dep
		}
		dep.Files[file] = string(data)
		return nil
	})
	if err != nil {
		return nil, err
	}

	deps := make([]Dependency, 0, len(byModule))
	for _, d := range byModule {
		deps = append(deps, *d)
	}
	sort.Slice(deps, func(i, j int) bool { return deps[i].Module < deps[j].Module })
	return deps, nil
}

// Text renders the notice and license of kubeswitch followed by the
// license files of every dependency. versions maps module paths to the
// versions built in; known ones are shown in the headings.
func Text(versions map[string]string) (string, error) {
	deps, err := Dependencies()
	if err != nil {
		return "", err
	}

	var b strings.Builder
	b.WriteString(strings.TrimSpace(Notice) + "\n\n")
	b.WriteString("kubeswitch is built with the following libraries; their licenses\n")
	b.WriteString("follow its own below.\n")
	for _, d := range deps {
		b.WriteString("  " + d.Module + versionSuffix(versions, d.Module) + "\n")
	}

	section(&b, "kubeswitch — LICENSE", Project)
	for _, d := range deps {
		names := make([]string, 0, len(d.Files))
		for name := range d.Files {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			section(&b, d.Module+versionSuffix(versions, d.Module)+" — "+name, d.Files[name])
		}
	}
	return b.String(), nil
}

func versionSuffix(versions map[string]string, module string) string {
	if v := versions[module]; v != "" {
		return " " + v
	}
	return ""
}

// section appends text under a ruled heading.
func section(b *strings.Builder, title, text string) {
	rule := strings.Repeat("=", 72)
	fmt.Fprintf(b, "\n%s\n%s\n%s\n\n%s\n", rule, title, rule, strings.TrimRight(text, "\n"))
}
