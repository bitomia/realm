package main

import (
	"fmt"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	clientPkg "github.com/bitomia/realm/cmd/client"
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
		client := clientPkg.NewClient()

		nodeImagesMap, err := client.GetAllImages()
		if err != nil {
			color.Red("Error %v\n", err)
			return
		}

		for _, nodeImages := range nodeImagesMap {
			if nodeImages.Error != "" {
				color.Red("Error in %s: %s\n", color.CyanString(nodeImages.Node), nodeImages.Error)
			} else {
				color.Blue("Images in %s\n", color.CyanString(nodeImages.Node))
				for _, image := range nodeImages.Images {
					log.Info("- %s\n", color.CyanString(image.Name))
				}
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
		client := clientPkg.NewClient()
		node := clientPkg.GetNode(args[0])
		if node == nil {
			color.Red("Cannot retrieve node %s\n", args[0])
		}

		color.Blue("Pulling image %s on %s\n", color.CyanString(args[1]), color.CyanString(args[0]))
		if err := client.PullImage(node.Url, args[1]); err != nil {
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
