package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	clientPkg "github.com/bitomia/realm/clients/go"
)

//nolint:unused
var authCmd = &cobra.Command{
	Use:                   "auth",
	Short:                 "Authorization module",
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Realm CLI. Use -h for help.")
	},
}

//nolint:unused
var initCmd = &cobra.Command{
	Use:                   "init",
	Short:                 "Init with a secret token",
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Print("Enter token: ")
		tokenBytes, err := term.ReadPassword(int(syscall.Stdin)) // nolint
		if err != nil {
			color.Red("\nError reading password: %v\n", err)
			return
		}
		token := string(tokenBytes)
		fmt.Println() // New line after token input

		// Save token to ~/.realmrc file
		if err := clientPkg.SaveAuthToken(token); err != nil {
			color.Yellow("Warning: Failed to save token to ~/.realmrc: %v\n", err)
			fmt.Println("\nTo use this token, set the REALM_BEARER environment variable:")
			fmt.Printf("export REALM_BEARER=%s\n", color.CyanString(token))
		} else {
			color.Green("Token saved.")
		}
	},
}

//nolint:unused
var logoutCmd = &cobra.Command{
	Use:                   "logout",
	Short:                 "Logout and remove authentication token",
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		// Check if a token is stored
		hasToken, err := clientPkg.HasAuthToken()
		if err != nil {
			color.Red("Error checking authentication token: %v\n", err)
			return
		}
		if !hasToken {
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
			color.Blue("Logout canceled.\n")
			return
		}

		// Remove the stored token
		if err := clientPkg.RemoveAuthToken(); err != nil {
			color.Red("Error removing token file: %v\n", err)
			return
		}

		color.Green("Successfully logged out. Authentication token removed.\n")
	},
}

// TODO auth module temporary disabled
func init() {
	// authCmd.AddCommand(initCmd)
	// authCmd.AddCommand(logoutCmd)
	// rootCmd.AddCommand(authCmd)
}
