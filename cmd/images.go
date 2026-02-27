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
		client := clientPkg.NewClient(cfg)

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

func init() {
	imagesCmd.AddCommand(listImages)
	rootCmd.AddCommand(imagesCmd)
}
