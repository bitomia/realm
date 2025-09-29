package main

import (
	"fmt"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/bitomia/realm/cmd/internal"
	"github.com/bitomia/realm/cmd/log"
)

var imagesCmd = &cobra.Command{
	Use:     "images",
	Aliases: []string{"i"},
	Short:   "Interface with images",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Realm CLI. Use -h for help.")
	},
}

var listImages = &cobra.Command{
	Use:   "ls",
	Short: "List all available images",
	Run: func(cmd *cobra.Command, args []string) {
		client := internal.NewClient()
		imagesPerNode, err := client.GetAllImages()
		if err != nil {
			color.Red("Error %v\n", err)
			return
		}
		for node, images := range imagesPerNode {
			color.Blue("Images in %s\n", color.CyanString(node))
			for _, image := range images {
				log.Info("- %s\n", color.CyanString(image.Name))
			}
		}
	},
}

var pullImage = &cobra.Command{
	Use:                   "pull [node] [image]",
	Short:                 "Pull a container image",
	Args:                  cobra.ExactArgs(2),
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		client := internal.NewClient()
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
