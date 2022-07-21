package commands

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/argoproj/argo-cd/v2/cmd/argocd/commands/headless"
	cmdutil "github.com/argoproj/argo-cd/v2/cmd/util"
	argocdclient "github.com/argoproj/argo-cd/v2/pkg/apiclient"
	"github.com/argoproj/argo-cd/v2/pkg/apiclient/applicationset"
	arogappsetv1 "github.com/argoproj/argo-cd/v2/pkg/apis/applicationset/v1alpha1"
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
		Short: "Create an ApplicationSet",
		Example: `
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
					FilePath:       fileUrl,
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
	return command
}

// NewApplicationListommand returns a new instance of an `argocd appset list` command
func NewApplicationSetListCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		output   string
		selector string
	)
	var command = &cobra.Command{
		Use:   "list",
		Short: "list of applicationSet",
		Example: `  # List all apps
  			argocd applicationset list`,
		Run: func(c *cobra.Command, args []string) {
			conn, appIf := headless.NewClientOrDie(clientOpts, c).NewApplicationSetClientOrDie()
			defer argoio.Close(conn)
			apps, err := appIf.List(context.Background(), &applicationset.ApplicationSetQuery{Selector: selector})
			errors.CheckError(err)
			appList := apps.Items

			switch output {
			case "yaml", "json":
				err := PrintResourceList(appList, output, false)
				errors.CheckError(err)
			case "name":
				printApplicationSetNames(appList)
			case "wide", "":
				printApplicationSetTable(appList, &output)
			default:
				errors.CheckError(fmt.Errorf("unknown output format: %s", output))
			}
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
				appSetCreateRequest := applicationset.ApplicationSetUpdateRequest{
					Applicationset: appset,
				}
				created, err := appIf.Update(context.Background(), &appSetCreateRequest)
				errors.CheckError(err)
				fmt.Printf("application set '%s' created\n", created.ObjectMeta.Name)
			}
			log.Printf("AppSet Create command %s", strings.Join(args, " "))
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

// Print simple list of application names
func printApplicationSetNames(apps []arogappsetv1.ApplicationSet) {
	for _, app := range apps {
		fmt.Println(app.Name)
	}
}

// Print table of application data
func printApplicationSetTable(apps []arogappsetv1.ApplicationSet, output *string) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	var fmtStr string
	headers := []interface{}{"NAME", "CLUSTER", "NAMESPACE", "PROJECT", "SYNCPOLICY", "CONDITIONS"}
	if *output == "wide" {
		fmtStr = "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n"
		headers = append(headers, "REPO", "PATH", "TARGET")
	} else {
		fmtStr = "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n"
	}
	_, _ = fmt.Fprintf(w, fmtStr, headers...)
	for _, app := range apps {
		vals := []interface{}{
			app.ObjectMeta.Name,
			app.ObjectMeta.Namespace,
			app.Spec.Template.Spec.Project,
			app.Spec.SyncPolicy,
			app.Status.Conditions,
		}
		_, _ = fmt.Fprintf(w, fmtStr, vals...)
	}
	_ = w.Flush()
}
