// Copyright 2026 Yannick Müller
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"

	"github.com/Yanni8/kubeswitch/internal/prompt"
	"github.com/Yanni8/kubeswitch/internal/store"
)

// pickKubeconfig lets the user choose one of the stored kubeconfigs from
// a menu. It returns a zero Kubeconfig when there are none. Swatches are
// escape sequences, so they are only added when the menu is shown on a
// terminal.
func pickKubeconfig(p *prompt.Prompter, st *store.Store, title string, swatches bool) (store.Kubeconfig, error) {
	configs, err := st.List()
	if err != nil || len(configs) == 0 {
		return store.Kubeconfig{}, err
	}
	opts := make([]prompt.Option, len(configs))
	for i, c := range configs {
		color := c.Color
		if sw := colorSwatch(c.Color); swatches && sw != "" {
			color += " " + sw
		}
		opts[i] = prompt.Option{
			Value: c.Name,
			Label: fmt.Sprintf("%s  (%s, %s)", c.Name, c.Environment, color),
		}
	}
	name, err := p.Select(title, opts)
	if err != nil {
		return store.Kubeconfig{}, err
	}
	for _, c := range configs {
		if c.Name == name {
			return c, nil
		}
	}
	return store.Kubeconfig{}, fmt.Errorf("kubeconfig %q %w", name, store.ErrNotFound)
}
