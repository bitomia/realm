package main

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var loginCmd = &cobra.Command{
	Use:                   "login",
	Short:                 "Login with a valid token",
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Print("Enter token: ")
		tokenBytes, err := term.ReadPassword(int(syscall.Stdin))
		if err != nil {
			color.Red("\nError reading password: %v\n", err)
			return
		}
		token := string(tokenBytes)
		fmt.Println() // New line after token input

		// Save token to ~/.realmrc file
		if err := saveTokenToFile(token); err != nil {
			color.Yellow("Warning: Failed to save token to ~/.realmrc: %v\n", err)
			fmt.Println("\nTo use this token, set the REALM_BEARER environment variable:")
			fmt.Printf("export REALM_BEARER=%s\n", color.CyanString(token))
		} else {
			color.Green("Token saved.")
		}
	},
}

func saveTokenToFile(token string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %v", err)
	}

	realmrcPath := filepath.Join(homeDir, ".realmrc")
	return os.WriteFile(realmrcPath, []byte(token), 0600)
}

func init() {
	rootCmd.AddCommand(loginCmd)
}
