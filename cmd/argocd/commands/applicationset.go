package commands

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/mattn/go-isatty"
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
	argocd appset list

	# Create an ApplicationSet
	argocd appset create "(filename.yaml)"

	# Delete an ApplicationSet
	argocd appset delete "(applicationset resource name)"
	`)
)

// NewAppSetCommand returns a new instance of an `argocd appset` command
func NewAppSetCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var command = &cobra.Command{
		Use:     "appset",
		Short:   "Manage applicationsets",
		Example: appSetExample,
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
	var command = &cobra.Command{
		Use:   "create",
		Short: "Create an ApplicationSet",
		Example: `
			argocd appset create <filename>
		`,
		Run: func(c *cobra.Command, args []string) {
			ctx := c.Context()

			if len(args) == 0 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
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
				}
				created, err := appIf.Create(ctx, &appSetCreateRequest)
				errors.CheckError(err)
				fmt.Printf("application set '%s' created\n", created.ObjectMeta.Name)
			}
		},
	}
	return command
}

// NewApplicationListommand returns a new instance of an `argocd appset list` command
func NewApplicationSetListCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		output   string
		selector string
		projects []string
	)
	var command = &cobra.Command{
		Use:   "list",
		Short: "list of applicationSet",
		Example: `  
			# List all appsets
  			argocd appset list
			
			# List apps by label, in this example we listing apps that are children of another app (aka app-of-apps)
			argocd app list -l app.kubernetes.io/instance=my-app
		`,
		Run: func(c *cobra.Command, args []string) {
			ctx := c.Context()

			conn, appIf := headless.NewClientOrDie(clientOpts, c).NewApplicationSetClientOrDie()
			defer argoio.Close(conn)
			appsets, err := appIf.List(ctx, &applicationset.ApplicationSetQuery{Selector: selector})
			errors.CheckError(err)
			appsetList := appsets.Items

			switch output {
			case "yaml", "json":
				err := PrintResourceList(appsetList, output, false)
				errors.CheckError(err)
			case "name":
				printApplicationSetNames(appsetList)
			case "wide", "":
				printApplicationSetTable(appsetList, &output)
			default:
				errors.CheckError(fmt.Errorf("unknown output format: %s", output))
			}
		},
	}
	command.Flags().StringVarP(&output, "output", "o", "wide", "Output format. One of: wide|name|json|yaml")
	command.Flags().StringVarP(&selector, "selector", "l", "", "List apps by label")
	command.Flags().StringArrayVarP(&projects, "project", "p", []string{}, "Filter by project name")

	return command
}

// NewApplicationSetUpdateCommand returns a new instance of an `argocd appset update` command
func NewApplicationSetUpdateCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var command = &cobra.Command{
		Use:   "update",
		Short: "Updates the given applicationSet",
		Run: func(c *cobra.Command, args []string) {
			ctx := c.Context()

			if len(args) == 0 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
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
				appsetUpdateRequest := applicationset.ApplicationSetUpdateRequest{
					Applicationset: appset,
				}
				updated, err := appIf.Update(ctx, &appsetUpdateRequest)
				errors.CheckError(err)
				fmt.Printf("application set '%s' updated\n", updated.Name)
			}
		},
	}
	return command
}

// NewApplicationSetDeleteCommand returns a new instance of an `argocd appset delete` command
func NewApplicationSetDeleteCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		noPrompt bool
	)
	var command = &cobra.Command{
		Use:   "delete",
		Short: "Delete an applicationSet",
		Example: `  
			# Delete an applicationset
			argocd appset delete APPNAME
		`,
		Run: func(c *cobra.Command, args []string) {
			ctx := c.Context()

			if len(args) == 0 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			conn, appIf := headless.NewClientOrDie(clientOpts, c).NewApplicationSetClientOrDie()
			defer argoio.Close(conn)
			var isTerminal bool = isatty.IsTerminal(os.Stdout.Fd()) || isatty.IsCygwinTerminal(os.Stdout.Fd())
			var isConfirmAll bool = false
			var numOfApps = len(args)
			var promptFlag = c.Flag("yes")
			if promptFlag.Changed && promptFlag.Value.String() == "true" {
				noPrompt = true
			}
			for _, appsetName := range args {
				appsetDeleteReq := applicationset.ApplicationSetDeleteRequest{
					Name: appsetName,
				}

				if isTerminal && !noPrompt {
					var confirmAnswer string = "n"
					var lowercaseAnswer string
					if numOfApps == 1 {
						fmt.Println("Are you sure you want to delete '" + appsetName + "' and all its resources? [y/n]")
						fmt.Scan(&confirmAnswer)
						lowercaseAnswer = strings.ToLower(confirmAnswer)
					} else {
						if !isConfirmAll {
							fmt.Println("Are you sure you want to delete '" + appsetName + "' and all its resources? [y/n/A] where 'A' is to delete all specified apps and their resources without prompting")
							fmt.Scan(&confirmAnswer)
							lowercaseAnswer = strings.ToLower(confirmAnswer)
							if lowercaseAnswer == "a" || lowercaseAnswer == "all" {
								lowercaseAnswer = "y"
								isConfirmAll = true
							}
						} else {
							lowercaseAnswer = "y"
						}
					}
					if lowercaseAnswer == "y" || lowercaseAnswer == "yes" {
						_, err := appIf.Delete(ctx, &appsetDeleteReq)
						errors.CheckError(err)
						fmt.Printf("applicationset '%s' deleted\n", appsetName)
					} else {
						fmt.Println("The command to delete '" + appsetName + "' was cancelled.")
					}
				} else {
					_, err := appIf.Delete(ctx, &appsetDeleteReq)
					errors.CheckError(err)
				}
			}
		},
	}
	command.Flags().BoolVarP(&noPrompt, "yes", "y", false, "Turn off prompting to confirm cascaded deletion of application resources")

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
	headers := []interface{}{"NAME", "NAMESPACE", "PROJECT", "SYNCPOLICY", "CONDITIONS"}
	if *output == "wide" {
		fmtStr = "%s\t%s\t%s\t%s\t%s\n"
		headers = append(headers, "REPO", "PATH", "TARGET")
	} else {
		fmtStr = "%s\t%s\t%s\t%s\t%s\n"
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
