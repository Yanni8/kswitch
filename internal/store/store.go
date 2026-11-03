// Package store persists the kubeconfigs added to kswitch together with
// their metadata (environment and terminal background color).
//
// Layout of the store directory:
//
//	state.json        name -> {environment, color, file}
//	configs/<slug>.yaml  the kubeconfigs as pasted, mode 0600
package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Environment classifies a kubeconfig.
type Environment string

const (
	EnvProd Environment = "prod"
	EnvDev  Environment = "dev"
)

// Entry is the metadata stored for one kubeconfig.
type Entry struct {
	Environment Environment `json:"environment"`
	Color       string      `json:"color"`
	// File is the kubeconfig's file name inside the configs directory.
	File string `json:"file"`
}

type state struct {
	Entries map[string]Entry `json:"entries"`
}

// Store reads and writes kswitch's on-disk data.
type Store struct {
	dir string
}

// Open returns the store at $KSWITCH_CONFIG_DIR, defaulting to the user
// config directory (~/.config/kswitch on Linux). Nothing is created on
// disk until the first write.
func Open() (*Store, error) {
	if dir := os.Getenv("KSWITCH_CONFIG_DIR"); dir != "" {
		return &Store{dir: dir}, nil
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("locating config dir: %w", err)
	}
	return &Store{dir: filepath.Join(base, "kswitch")}, nil
}

func (s *Store) statePath() string  { return filepath.Join(s.dir, "state.json") }
func (s *Store) configsDir() string { return filepath.Join(s.dir, "configs") }

const maxNameLen = 64

// ValidateName checks that name is usable as a human-readable entry name.
func ValidateName(name string) error {
	if name == "" {
		return errors.New("name must not be empty")
	}
	if utf8.RuneCountInString(name) > maxNameLen {
		return fmt.Errorf("name is too long (max %d characters)", maxNameLen)
	}
	for _, r := range name {
		if !unicode.IsPrint(r) {
			return errors.New("name must not contain control characters or line breaks")
		}
	}
	return nil
}

// Kubeconfig is a stored kubeconfig together with its metadata.
type Kubeconfig struct {
	Name        string
	Environment Environment
	Color       string
	Path        string // where the kubeconfig file is stored
}

// List returns all stored kubeconfigs, sorted by name (case-insensitively).
func (s *Store) List() ([]Kubeconfig, error) {
	st, err := s.load()
	if err != nil {
		return nil, err
	}
	list := make([]Kubeconfig, 0, len(st.Entries))
	for name, e := range st.Entries {
		list = append(list, Kubeconfig{
			Name:        name,
			Environment: e.Environment,
			Color:       e.Color,
			Path:        filepath.Join(s.configsDir(), e.File),
		})
	}
	sort.Slice(list, func(i, j int) bool {
		a, b := strings.ToLower(list[i].Name), strings.ToLower(list[j].Name)
		if a != b {
			return a < b
		}
		return list[i].Name < list[j].Name
	})
	return list, nil
}

// Exists reports whether an entry with this name (compared
// case-insensitively) is already stored.
func (s *Store) Exists(name string) (bool, error) {
	st, err := s.load()
	if err != nil {
		return false, err
	}
	return findEntry(st, name) != "", nil
}

// Add stores a kubeconfig under name and returns the path it was written
// to. The file is created with mode 0600 since kubeconfigs usually
// contain credentials.
func (s *Store) Add(name string, env Environment, color string, kubecfg []byte) (string, error) {
	if err := ValidateName(name); err != nil {
		return "", err
	}
	if env != EnvProd && env != EnvDev {
		return "", fmt.Errorf("invalid environment %q", env)
	}
	if color == "" {
		return "", errors.New("color must not be empty")
	}

	st, err := s.load()
	if err != nil {
		return "", err
	}
	if existing := findEntry(st, name); existing != "" {
		return "", fmt.Errorf("kubeconfig %q already exists", existing)
	}

	if err := os.MkdirAll(s.configsDir(), 0o700); err != nil {
		return "", err
	}
	file := s.uniqueFile(slug(name))
	path := filepath.Join(s.configsDir(), file)
	if err := os.WriteFile(path, kubecfg, 0o600); err != nil {
		return "", err
	}

	st.Entries[name] = Entry{Environment: env, Color: color, File: file}
	if err := s.save(st); err != nil {
		os.Remove(path) // roll back so a retry starts clean
		return "", err
	}
	return path, nil
}

// findEntry returns the stored name matching name case-insensitively,
// or "" if there is none.
func findEntry(st *state, name string) string {
	for stored := range st.Entries {
		if strings.EqualFold(stored, name) {
			return stored
		}
	}
	return ""
}

var nonSlug = regexp.MustCompile(`[^a-z0-9._-]+`)

// slug turns a human-readable name into a safe file name stem.
func slug(name string) string {
	s := nonSlug.ReplaceAllString(strings.ToLower(name), "-")
	s = strings.Trim(s, "-._")
	if s == "" {
		return "kubeconfig"
	}
	return s
}

// uniqueFile returns "<base>.yaml", or "<base>-N.yaml" if that file
// already exists in the configs directory.
func (s *Store) uniqueFile(base string) string {
	for i := 1; ; i++ {
		file := base + ".yaml"
		if i > 1 {
			file = fmt.Sprintf("%s-%d.yaml", base, i)
		}
		if _, err := os.Stat(filepath.Join(s.configsDir(), file)); err != nil {
			return file
		}
	}
}

func (s *Store) load() (*state, error) {
	st := &state{Entries: map[string]Entry{}}
	data, err := os.ReadFile(s.statePath())
	if errors.Is(err, os.ErrNotExist) {
		return st, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, st); err != nil {
		return nil, fmt.Errorf("reading %s: %w", s.statePath(), err)
	}
	if st.Entries == nil {
		st.Entries = map[string]Entry{}
	}
	return st, nil
}

func (s *Store) save(st *state) error {
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return err
	}
	return os.WriteFile(s.statePath(), append(data, '\n'), 0o600)
}
