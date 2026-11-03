// Copyright 2026 Yannick Müller
// SPDX-License-Identifier: Apache-2.0

package kubeconfig

import (
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

// RelativePaths returns the certificate, key, and token files a
// kubeconfig refers to by relative path. kubectl resolves those against
// the kubeconfig's own directory, so a copy stored elsewhere cannot find
// them.
func RelativePaths(data []byte) []string {
	var doc map[string]any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil
	}
	seen := map[string]bool{}
	collect := func(items []map[string]any, section string, keys ...string) {
		for _, item := range items {
			m := sub(item, section)
			for _, k := range keys {
				if p, ok := m[k].(string); ok && p != "" && !filepath.IsAbs(p) {
					seen[p] = true
				}
			}
		}
	}
	collect(entries(doc["clusters"]), "cluster", "certificate-authority")
	collect(entries(doc["users"]), "user", "client-certificate", "client-key", "tokenFile")

	paths := make([]string, 0, len(seen))
	for p := range seen {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	return paths
}
