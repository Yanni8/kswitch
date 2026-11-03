package kubeconfig

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// DefaultPaths returns the kubeconfig files kubectl uses: those listed
// in $KUBECONFIG, or ~/.kube/config.
func DefaultPaths() ([]string, error) {
	if env := os.Getenv("KUBECONFIG"); env != "" {
		var paths []string
		for _, p := range filepath.SplitList(env) {
			if p != "" {
				paths = append(paths, p)
			}
		}
		if len(paths) > 0 {
			return paths, nil
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	return []string{filepath.Join(home, ".kube", "config")}, nil
}

// Context is one context of a kubeconfig, split out as a kubeconfig of
// its own.
type Context struct {
	Name    string
	Cluster string
	Server  string
	Config  []byte // a kubeconfig holding only this context, its cluster, and its user
}

// Split reads kubeconfig files, merged the way kubectl merges them (the
// first definition of a name wins), and returns a standalone kubeconfig
// for every context, in order. Contexts whose cluster is not defined are
// left out. Relative certificate and key paths are made absolute, so the
// results work from any directory.
func Split(paths ...string) ([]Context, error) {
	var m merged
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		var doc map[string]any
		if err := yaml.Unmarshal(data, &doc); err != nil {
			return nil, fmt.Errorf("%s: not valid YAML: %w", path, err)
		}
		m.add(doc, filepath.Dir(path))
	}

	var contexts []Context
	for _, ctx := range m.contexts {
		name, _ := ctx["name"].(string)
		ref := sub(ctx, "context")
		clusterName, _ := ref["cluster"].(string)
		cluster := find(m.clusters, clusterName)
		if cluster == nil {
			continue
		}
		doc := map[string]any{
			"apiVersion":      "v1",
			"kind":            "Config",
			"current-context": name,
			"clusters":        []any{cluster},
			"contexts":        []any{ctx},
			"users":           []any{},
		}
		if userName, _ := ref["user"].(string); userName != "" {
			if user := find(m.users, userName); user != nil {
				doc["users"] = []any{user}
			}
		}
		if m.preferences != nil {
			doc["preferences"] = m.preferences
		}
		out, err := marshal(doc)
		if err != nil {
			return nil, err
		}
		server, _ := sub(cluster, "cluster")["server"].(string)
		contexts = append(contexts, Context{Name: name, Cluster: clusterName, Server: server, Config: out})
	}
	return contexts, nil
}

// marshal encodes doc as YAML with the two-space indentation kubeconfigs
// conventionally use.
func marshal(doc map[string]any) ([]byte, error) {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(doc); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// merged accumulates the named entries of several kubeconfig files.
type merged struct {
	clusters, contexts, users []map[string]any
	preferences               any
}

func (m *merged) add(doc map[string]any, dir string) {
	m.clusters = appendNew(m.clusters, entries(doc["clusters"]), func(e map[string]any) {
		absolutize(sub(e, "cluster"), dir, "certificate-authority")
	})
	m.users = appendNew(m.users, entries(doc["users"]), func(e map[string]any) {
		absolutize(sub(e, "user"), dir, "client-certificate", "client-key", "tokenFile")
	})
	m.contexts = appendNew(m.contexts, entries(doc["contexts"]), nil)
	if m.preferences == nil {
		m.preferences = doc["preferences"]
	}
}

// entries returns the mappings of a kubeconfig list such as "clusters".
func entries(v any) []map[string]any {
	items, _ := v.([]any)
	var out []map[string]any
	for _, item := range items {
		if e, ok := item.(map[string]any); ok {
			out = append(out, e)
		}
	}
	return out
}

// appendNew adds the entries whose name is not in dst yet, after fixing
// them up with fix.
func appendNew(dst, src []map[string]any, fix func(map[string]any)) []map[string]any {
	for _, e := range src {
		name, _ := e["name"].(string)
		if name == "" || find(dst, name) != nil {
			continue
		}
		if fix != nil {
			fix(e)
		}
		dst = append(dst, e)
	}
	return dst
}

func find(entries []map[string]any, name string) map[string]any {
	for _, e := range entries {
		if e["name"] == name {
			return e
		}
	}
	return nil
}

// sub returns the nested mapping under key, or nil.
func sub(e map[string]any, key string) map[string]any {
	m, _ := e[key].(map[string]any)
	return m
}

// absolutize makes relative paths stored under keys absolute, relative
// to dir.
func absolutize(m map[string]any, dir string, keys ...string) {
	for _, k := range keys {
		if p, ok := m[k].(string); ok && p != "" && !filepath.IsAbs(p) {
			m[k] = filepath.Join(dir, p)
		}
	}
}
