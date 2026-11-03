package store

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func openTemp(t *testing.T) (*Store, string) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("KSWITCH_CONFIG_DIR", dir)
	s, err := Open()
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	return s, dir
}

func TestOpenDefaultsToUserConfigDir(t *testing.T) {
	t.Setenv("KSWITCH_CONFIG_DIR", "")
	t.Setenv("XDG_CONFIG_HOME", "/xdg")
	s, err := Open()
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if want := filepath.Join("/xdg", "kswitch"); s.dir != want {
		t.Errorf("dir = %q, want %q", s.dir, want)
	}
}

func TestAddAndExists(t *testing.T) {
	s, dir := openTemp(t)

	path, err := s.Add("Prod EU", EnvProd, "red", []byte("apiVersion: v1\n"))
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if want := filepath.Join(dir, "configs", "prod-eu.yaml"); path != want {
		t.Errorf("path = %q, want %q", path, want)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("kubeconfig mode = %o, want 600", info.Mode().Perm())
	}
	if dirInfo, _ := os.Stat(filepath.Dir(path)); dirInfo.Mode().Perm() != 0o700 {
		t.Errorf("configs dir mode = %o, want 700", dirInfo.Mode().Perm())
	}
	if data, _ := os.ReadFile(path); string(data) != "apiVersion: v1\n" {
		t.Errorf("stored content %q", data)
	}

	for name, want := range map[string]bool{"Prod EU": true, "prod eu": true, "other": false} {
		if got, err := s.Exists(name); err != nil || got != want {
			t.Errorf("Exists(%q) = %v, %v; want %v", name, got, err, want)
		}
	}

	// A fresh store reads the same state back from disk.
	again, _ := Open()
	if exists, _ := again.Exists("Prod EU"); !exists {
		t.Error("entry not persisted")
	}
}

func TestListSortsByName(t *testing.T) {
	s, dir := openTemp(t)
	if list, err := s.List(); err != nil || len(list) != 0 {
		t.Fatalf("empty store: got %v, %v", list, err)
	}

	for _, name := range []string{"beta", "Alpha", "gamma"} {
		if _, err := s.Add(name, EnvDev, "blue", []byte("a: 1\n")); err != nil {
			t.Fatal(err)
		}
	}
	list, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	var names []string
	for _, k := range list {
		names = append(names, k.Name)
	}
	if got, want := strings.Join(names, ","), "Alpha,beta,gamma"; got != want {
		t.Errorf("names = %q, want %q", got, want)
	}
	want := Kubeconfig{Name: "Alpha", Environment: EnvDev, Color: "blue", Path: filepath.Join(dir, "configs", "alpha.yaml")}
	if list[0] != want {
		t.Errorf("first entry = %+v, want %+v", list[0], want)
	}
}

func TestGetAndRemove(t *testing.T) {
	s, _ := openTemp(t)
	path, err := s.Add("Prod EU", EnvProd, "red", []byte("a: 1\n"))
	if err != nil {
		t.Fatal(err)
	}

	k, err := s.Get("prod eu")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if want := (Kubeconfig{Name: "Prod EU", Environment: EnvProd, Color: "red", Path: path}); k != want {
		t.Errorf("Get = %+v, want %+v", k, want)
	}
	if _, err := s.Get("nope"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Get(nope) = %v, want ErrNotFound", err)
	}

	if err := s.Remove("PROD EU"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("kubeconfig file should be deleted (stat: %v)", err)
	}
	if list, _ := s.List(); len(list) != 0 {
		t.Errorf("entry still listed: %+v", list)
	}
	if err := s.Remove("Prod EU"); !errors.Is(err, ErrNotFound) {
		t.Errorf("second Remove = %v, want ErrNotFound", err)
	}
}

func TestAddRejectsDuplicateName(t *testing.T) {
	s, _ := openTemp(t)
	if _, err := s.Add("Prod EU", EnvProd, "red", []byte("a: 1\n")); err != nil {
		t.Fatal(err)
	}
	_, err := s.Add("PROD EU", EnvDev, "blue", []byte("a: 2\n"))
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Errorf("got %v, want duplicate error", err)
	}
}

func TestAddKeepsFileNamesUnique(t *testing.T) {
	s, dir := openTemp(t)
	// Different names, same slug.
	if _, err := s.Add("Prod EU", EnvProd, "red", []byte("a: 1\n")); err != nil {
		t.Fatal(err)
	}
	path, err := s.Add("prod-eu", EnvDev, "blue", []byte("a: 2\n"))
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(dir, "configs", "prod-eu-2.yaml"); path != want {
		t.Errorf("path = %q, want %q", path, want)
	}
	if data, _ := os.ReadFile(filepath.Join(dir, "configs", "prod-eu.yaml")); string(data) != "a: 1\n" {
		t.Errorf("first kubeconfig was overwritten: %q", data)
	}
}

func TestAddValidates(t *testing.T) {
	s, dir := openTemp(t)
	tests := []struct {
		name  string
		entry string
		env   Environment
		color string
	}{
		{"bad environment", "x", "staging", "red"},
		{"empty color", "x", EnvProd, ""},
		{"bad name", "", EnvProd, "red"},
	}
	for _, tt := range tests {
		if _, err := s.Add(tt.entry, tt.env, tt.color, []byte("a: 1\n")); err == nil {
			t.Errorf("%s: expected error", tt.name)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "state.json")); !os.IsNotExist(err) {
		t.Errorf("state should not exist after failed adds (stat: %v)", err)
	}
}

func TestValidateName(t *testing.T) {
	for _, ok := range []string{"dev", "My Cluster (EU)", "äöü", strings.Repeat("x", 64)} {
		if err := ValidateName(ok); err != nil {
			t.Errorf("ValidateName(%q) = %v, want nil", ok, err)
		}
	}
	for _, bad := range []string{"", "a\nb", "a\tb", strings.Repeat("x", 65)} {
		if err := ValidateName(bad); err == nil {
			t.Errorf("ValidateName(%q) = nil, want error", bad)
		}
	}
}

func TestSlug(t *testing.T) {
	tests := map[string]string{
		"Prod EU":                               "prod-eu",
		"arn:aws:eks:eu-west-1:123:cluster/foo": "arn-aws-eks-eu-west-1-123-cluster-foo",
		"  Spaces  ":                            "spaces",
		".hidden":                               "hidden",
		"***":                                   "kubeconfig",
	}
	for in, want := range tests {
		if got := slug(in); got != want {
			t.Errorf("slug(%q) = %q, want %q", in, got, want)
		}
	}
}
