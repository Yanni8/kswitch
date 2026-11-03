// Copyright 2026 Yannick Müller
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Yanni8/kubeswitch/internal/store"
)

// completeKubeconfigNames offers the stored kubeconfig names, with their
// environment and color as description, for a command's first argument.
func completeKubeconfigNames(_ *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	st, err := store.Open()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	configs, err := st.List()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	var names []string
	for _, c := range configs {
		if strings.HasPrefix(strings.ToLower(c.Name), strings.ToLower(toComplete)) {
			names = append(names, fmt.Sprintf("%s\t%s, %s", c.Name, c.Environment, c.Color))
		}
	}
	return names, cobra.ShellCompDirectiveNoFileComp
}
