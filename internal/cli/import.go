package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Yanni8/kubeswitch/internal/eks"
	"github.com/Yanni8/kubeswitch/internal/kubeconfig"
	"github.com/Yanni8/kubeswitch/internal/prompt"
	"github.com/Yanni8/kubeswitch/internal/store"
)

func newImportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "import",
		Short: "Import a kubeconfig from an external provider",
		Long: `Import a kubeconfig from an external provider instead of pasting it.

Each provider is a subcommand; see "kubeswitch import <provider> --help".`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(newImportAWSCmd())
	return cmd
}

func newImportAWSCmd() *cobra.Command {
	var o eks.Options
	cmd := &cobra.Command{
		Use:   "aws [cluster]",
		Short: "Import the kubeconfig of an AWS EKS cluster",
		Long: `Import the kubeconfig of an AWS EKS cluster using the AWS CLI.

Without a cluster name, the clusters in the region are listed to pick
from. The kubeconfig is fetched with "aws eks update-kubeconfig", so the
AWS CLI must be installed and configured; --region, --profile, and
--role-arn are passed through to it. kubeswitch then asks for a name
(defaulting to the cluster name), the environment, and the color, as
"add" does.

Authentication to the cluster goes through the AWS CLI as well ("aws eks
get-token"); the profile, if given, is remembered in the kubeconfig.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runImportAWS(cmd, args, o)
		},
	}
	cmd.Flags().StringVar(&o.Region, "region", "", "AWS region of the cluster (default: the AWS CLI's configured region)")
	cmd.Flags().StringVar(&o.Profile, "profile", "", "AWS CLI profile to use")
	cmd.Flags().StringVar(&o.RoleARN, "role-arn", "", "IAM role to assume when authenticating to the cluster")
	return cmd
}

func runImportAWS(cmd *cobra.Command, args []string, o eks.Options) error {
	out := cmd.OutOrStdout()

	st, err := store.Open()
	if err != nil {
		return err
	}
	p := prompt.New(cmd.InOrStdin(), out)

	var cluster string
	if len(args) == 1 {
		cluster = args[0]
	} else {
		clusters, err := eks.ListClusters(o)
		if err != nil {
			return err
		}
		if len(clusters) == 0 {
			fmt.Fprintln(out, "No EKS clusters found"+regionSuffix(o)+".")
			return nil
		}
		opts := make([]prompt.Option, len(clusters))
		for i, c := range clusters {
			opts[i] = prompt.Option{Value: c, Label: c}
		}
		if cluster, err = p.Select("Which EKS cluster do you want to import?", opts); err != nil {
			return err
		}
	}

	fmt.Fprintf(out, "Fetching the kubeconfig for EKS cluster %q%s...\n", cluster, regionSuffix(o))
	raw, err := eks.Kubeconfig(cluster, o)
	if err != nil {
		return err
	}
	if _, err := kubeconfig.Parse(raw); err != nil {
		return fmt.Errorf("the AWS CLI produced an unexpected kubeconfig: %w", err)
	}

	return addKubeconfig(p, out, st, raw, cluster)
}

func regionSuffix(o eks.Options) string {
	if o.Region == "" {
		return ""
	}
	return " in " + o.Region
}
