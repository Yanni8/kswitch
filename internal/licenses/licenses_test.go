// Copyright 2026 Yannick Müller
// SPDX-License-Identifier: Apache-2.0

package licenses

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestEmbeddedCopiesMatchRepositoryFiles(t *testing.T) {
	for file, embedded := range map[string]string{"LICENSE": Project, "NOTICE": Notice} {
		repo, err := os.ReadFile("../../" + file)
		if err != nil {
			t.Fatalf("%s: %v", file, err)
		}
		if string(repo) != embedded {
			t.Errorf("internal/licenses/%s differs from the repository's %s; run \"make licenses\"", file, file)
		}
	}
	if !strings.Contains(Project, "Apache License") || !strings.Contains(Project, "Version 2.0") {
		t.Error("the project license should be the Apache License 2.0")
	}
	if !strings.Contains(Notice, "Copyright") {
		t.Error("the notice should carry the copyright line")
	}
}

// Every Go and shell file in the repository carries the copyright header
// that "make headers" adds.
func TestSourceFilesCarryTheLicenseHeader(t *testing.T) {
	var missing []string
	err := filepath.WalkDir("../..", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "bin", "dist", "third_party":
				return filepath.SkipDir
			}
			return nil
		}
		if ext := filepath.Ext(p); ext != ".go" && ext != ".sh" {
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		head := string(data[:min(len(data), 512)])
		if !strings.Contains(head, "SPDX-License-Identifier: Apache-2.0") || !strings.Contains(head, "Copyright") {
			missing = append(missing, strings.TrimPrefix(p, "../../"))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(missing) > 0 {
		t.Errorf("files without the copyright header (run \"make headers\"):\n  %s", strings.Join(missing, "\n  "))
	}
}

func TestDependencies(t *testing.T) {
	deps, err := Dependencies()
	if err != nil {
		t.Fatalf("Dependencies: %v", err)
	}
	if len(deps) == 0 {
		t.Fatal("no dependency licenses embedded; run \"make licenses\"")
	}

	modules := make([]string, len(deps))
	for i, d := range deps {
		modules[i] = d.Module
		if len(d.Files) == 0 {
			t.Errorf("%s has no license files", d.Module)
		}
		for name, text := range d.Files {
			if strings.TrimSpace(text) == "" {
				t.Errorf("%s/%s is empty", d.Module, name)
			}
		}
	}
	if !sort.StringsAreSorted(modules) {
		t.Errorf("dependencies are not sorted: %v", modules)
	}
	for _, want := range []string{"github.com/spf13/cobra", "gopkg.in/yaml.v3", "golang.org/x/term"} {
		if sort.SearchStrings(modules, want) >= len(modules) || modules[sort.SearchStrings(modules, want)] != want {
			t.Errorf("missing license for %s", want)
		}
	}
}

func TestText(t *testing.T) {
	text, err := Text(map[string]string{"github.com/spf13/cobra": "v1.10.2"})
	if err != nil {
		t.Fatalf("Text: %v", err)
	}
	for _, want := range []string{
		strings.TrimSpace(Notice),
		"  github.com/spf13/cobra v1.10.2\n",
		"kubeswitch — LICENSE",
		"github.com/spf13/cobra v1.10.2 — LICENSE.txt",
		"golang.org/x/term — LICENSE", // no version known
	} {
		if !strings.Contains(text, want) {
			t.Errorf("text lacks %q", want)
		}
	}
	if strings.Index(text, "kubeswitch — LICENSE") > strings.Index(text, "github.com/spf13/cobra v1.10.2 — LICENSE.txt") {
		t.Error("the project's license must come before the dependencies'")
	}
}
