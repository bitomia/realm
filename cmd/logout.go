package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var logoutCmd = &cobra.Command{
	Use:                   "logout",
	Short:                 "Logout and remove authentication token",
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			color.Red("Error getting home directory: %v\n", err)
			return
		}

		realmrcPath := filepath.Join(homeDir, ".realmrc")

		// Check if .realmrc file exists
		if _, err := os.Stat(realmrcPath); os.IsNotExist(err) {
			color.Blue("No authentication token found. Already logged out.\n")
			return
		}

		// Ask for confirmation
		fmt.Print("Are you sure you want to logout? This will remove your authentication token. (y/N): ")
		reader := bufio.NewReader(os.Stdin)
		response, err := reader.ReadString('\n')
		if err != nil {
			color.Red("Error reading input: %v\n", err)
			return
		}

		response = strings.TrimSpace(strings.ToLower(response))
		if response != "y" && response != "yes" {
			color.Blue("Logout cancelled.\n")
			return
		}

		// Remove the .realmrc file
		if err := os.Remove(realmrcPath); err != nil {
			color.Red("Error removing token file: %v\n", err)
			return
		}

		color.Green("Successfully logged out. Authentication token removed.\n")
	},
}

func init() {
	rootCmd.AddCommand(logoutCmd)
}
