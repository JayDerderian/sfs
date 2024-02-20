package cmd

import (
	"fmt"

	"github.com/sfs/pkg/client"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

/*
Command for adding a new file or directory to the local SFS client service
*/

var (
	item string

	addCmd = &cobra.Command{
		Use:   "add",
		Short: "Add a new file or directory to the local SFS client service",
		Run:   RunAddCmd,
	}
)

func init() {
	addCmd.Flags().StringVarP(&item, "item", "i", "", "Path to the new file or directory")

	viper.BindPFlag("item", addCmd.Flags().Lookup("item"))

	clientCmd.AddCommand(addCmd)
}

func RunAddCmd(cmd *cobra.Command, args []string) {
	path, _ := cmd.Flags().GetString("item")
	if path == "" {
		showerr(fmt.Errorf("no path specified"))
		return
	}
	c, err := client.LoadClient(false)
	if err != nil {
		showerr(err)
		return
	}
	if err := c.AddItem(path); err != nil {
		showerr(err)
		return
	}
}