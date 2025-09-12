package main

import (
	"fmt"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var imagesCmd = &cobra.Command{
	Use:   "images",
	Short: "Interface with images",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Realm CLI. Use -h for help.")
	},
}

var listImages = &cobra.Command{
	Use:   "ls",
	Short: "List all available images",
	Run: func(cmd *cobra.Command, args []string) {
		client := NewClient()
		imagesPerHost, err := client.GetAllImages()
		if err != nil {
			color.Red("Error %v\n", err)
			return
		}
		for host, images := range imagesPerHost {
			color.Green("Images in %s:\n", host)
			for _, image := range images {
				fmt.Printf("- %s\n", color.CyanString(image.Name))
			}
		}
	},
}

var pullImage = &cobra.Command{
	Use:                   "pull [host] [image]",
	Short:                 "Pull a container image",
	Args:                  cobra.ExactArgs(2),
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		client := NewClient()
		color.Blue("Pulling image %s on %s\n", color.CyanString(args[1]), color.CyanString(args[0]))
		if err := client.PullImage(args[0], args[1]); err != nil {
			color.Red("Error pulling image: %v\n", err)
		} else {
			color.Green("Successfully pulled image %s\n", color.CyanString(args[1]))
		}
	},
}

func init() {
	imagesCmd.AddCommand(listImages)
	imagesCmd.AddCommand(pullImage)
	rootCmd.AddCommand(imagesCmd)
}
