package cmd

import (
	"fmt"
	"os"

	"github.com/sfs/pkg/client"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

/*
Command for removing files or directories from the SFS filesystem
*/

var (
	RemoveCmd = &cobra.Command{
		Use:   "remove",
		Short: "Remove files or directories from the SFS filesystem",
		Run:   removeCmd,
	}
)

func init() {
	flags := FlagPole{}
	RemoveCmd.PersistentFlags().StringVarP(&flags.path, "path", "p", "", "Remove files or directories from the SFS filesystem using their absolute paths")
	RemoveCmd.PersistentFlags().BoolVarP(&flags.delete, "delete", "d", false, "set to true to delete, false to just stop monitoring")

	viper.BindPFlag("path", RemoveCmd.Flags().Lookup("path"))
	viper.BindPFlag("delete", RemoveCmd.Flags().Lookup("delete"))

	drvCmd.AddCommand(RemoveCmd)
}

func removeCmd(cmd *cobra.Command, args []string) {
	path, _ := cmd.Flags().GetString("path")
	delete, _ := cmd.Flags().GetBool("delete")
	if path == "" {
		showerr(fmt.Errorf("path was not provided"))
		return
	}
	c, err := client.LoadClient(false)
	if err != nil {
		showerr(fmt.Errorf("failed to initialize service: %v", err))
	}
	// move to recycle bin
	if err := c.RemoveItem(path); err != nil {
		showerr(fmt.Errorf("failed to remove item: %v", err))
	}
	// hard delete
	if delete {
		if err := os.Remove(path); err != nil {
			showerr(fmt.Errorf("failed to remove file: %v", err))
		}
	}
}
