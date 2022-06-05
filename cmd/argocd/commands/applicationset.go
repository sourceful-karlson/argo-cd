package commands

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/argoproj/argo-cd/v2/cmd/argocd/commands/headless"
	cmdutil "github.com/argoproj/argo-cd/v2/cmd/util"
	argocdclient "github.com/argoproj/argo-cd/v2/pkg/apiclient"
	"github.com/argoproj/argo-cd/v2/pkg/apiclient/applicationset"
	"github.com/argoproj/argo-cd/v2/util/errors"
	argoio "github.com/argoproj/argo-cd/v2/util/io"
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
		appOpts cmdutil.AppOptions
		upsert  bool
	)
	var command = &cobra.Command{
		Use:   "create ApplicationSet",
		Short: "Create an ApplicationSet",
		Example: `
		# Create the above ApplicationSet
		argocd appset create cluster-addons.yaml
	
`,
		Run: func(c *cobra.Command, args []string) {
			argocdClient := headless.NewClientOrDie(clientOpts, c)
			fileUrl := args[0]
			appsets, err := cmdutil.ConstructApplicationSet(fileUrl)
			errors.CheckError(err)

			for _, appset := range appsets {
				if appset.Name == "" {
					c.HelpFunc()(c, args)
					os.Exit(1)
				}

				conn, appIf := argocdClient.NewApplicationSetClientOrDie()
				defer argoio.Close(conn)
				appSetCreateRequest := applicationset.ApplicationSetCreateRequest{
					Applicationset: appset,
					Upsert:         upsert,
					Validate:       appOpts.Validate,
				}
				created, err := appIf.Create(context.Background(), &appSetCreateRequest)
				errors.CheckError(err)
				fmt.Printf("application set '%s' created\n", created.ObjectMeta.Name)
			}
			log.Printf("AppSet Create command %s", strings.Join(args, " "))
		},
	}
	// command.Flags().StringVarP(&fileURL, "file", "f", "", "Filename or URL to Kubernetes manifests for the ApplicationSet")
	// err := command.Flags().SetAnnotation("file", cobra.BashCompFilenameExt, []string{"json", "yaml", "yml"})
	// if err != nil {
	// 	log.Fatal(err)
	// }
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
			log.Printf("AppSet List command %s", strings.Join(args, " "))
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
			log.Printf("AppSet Update command %s", strings.Join(args, " "))
			log.Printf("ApplicationSet Name %s", appsetName)
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
			log.Printf("AppSet Delete command %s", strings.Join(args, " "))
			log.Printf("ApplicationSet Name %s", appsetName)
			os.Exit(1)
		},
	}
	command.Flags().StringVar(&appsetName, "applicationset-name", "foreground", "")
	return command
}
