package commands

import (
	"os"

	"github.com/spf13/cobra"

	argocdclient "github.com/argoproj/argo-cd/v2/pkg/apiclient"
	"github.com/argoproj/argo-cd/v2/util/templates"
)

var (
	appSetExample = templates.Examples(`
	# List all the applications.
	argocd appsets list
	`)
)

// NewAppSetCommand returns a new instance of an `argocd appset` command
func NewAppSetCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var command = &cobra.Command{
		Use:     "applicationset",
		Short:   "Manage applicationsets",
		Example: appExample,
		Run: func(c *cobra.Command, args []string) {
			c.HelpFunc()(c, args)
			os.Exit(1)
		},
	}
	command.AddCommand(NewApplicationSetCreateCommand(clientOpts))
	command.AddCommand(NewApplicationSetListCommand(clientOpts))
	command.AddCommand(NewApplicationSetUpdateCommand(clientOpts))
	command.AddCommand(NewApplicationSetDeleteCommand(clientOpts))
	return command
}

// NewApplicationCreateCommand returns a new instance of an `argocd appset create` command
func NewApplicationSetCreateCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		fileURL string
	)
	var command = &cobra.Command{
		Use:   "create ApplicationSet",
		Short: "Create an ApplicationSet",
		Example: `
		# Create the above ApplicationSet
		argocd appset create cluster-addons.yaml
	
`,
		Run: func(c *cobra.Command, args []string) {
			c.HelpFunc()(c, args)
			os.Exit(1)
		},
	}
	command.Flags().StringVarP(&fileURL, "file", "f", "", "Filename or URL to Kubernetes manifests for the ApplicationSet")

	return command
}

// NewApplicationListommand returns a new instance of an `argocd appset list` command
func NewApplicationSetListCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var ()
	var command = &cobra.Command{
		Use:   "list",
		Short: "list of applicationSet",
		Example: `  # List all apps
  argocd appset list`,
		Run: func(c *cobra.Command, args []string) {
			c.HelpFunc()(c, args)
			os.Exit(1)
		},
	}

	return command
}

// NewApplicationSetUpdateCommand returns a new instance of an `argocd appset update` command
func NewApplicationSetUpdateCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		appsetName string
	)
	var command = &cobra.Command{
		Use:   "update ApplicationSet",
		Short: "Updates the given applicationSet",
		Run: func(c *cobra.Command, args []string) {
			c.HelpFunc()(c, args)
			os.Exit(1)
		},
	}
	command.Flags().StringVar(&appsetName, "appset-name", "/", "Name of the appset")
	return command
}

// NewApplicationSetDeleteCommand returns a new instance of an `argocd appset delete` command
func NewApplicationSetDeleteCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		appsetName string
	)
	var command = &cobra.Command{
		Use:   "delete ApplicationSet",
		Short: "Delete an applicationSet",
		Run: func(c *cobra.Command, args []string) {
			c.HelpFunc()(c, args)
			os.Exit(1)
		},
	}
	command.Flags().StringVar(&appsetName, "applicationset-name", "foreground", "")
	return command
}
