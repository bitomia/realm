package main

// import (
// 	"bufio"
// 	"fmt"
// 	"os"
// 	"strings"

// 	"github.com/fatih/color"
// 	"github.com/spf13/cobra"

// 	"github.com/bitomia/realm/internal"
// 	"github.com/bitomia/realm/internal/config"
// )

// var bootCmd = &cobra.Command{
// 	Use:                   "boot",
// 	Short:                 "Interface to startup and shutdown cluster",
// 	DisableFlagsInUseLine: true,
// 	Run: func(cmd *cobra.Command, args []string) {
// 		fmt.Println("Realm CLI. Use -h for help.")
// 	},
// }

// var startConfig = &cobra.Command{
// 	Use:                   "start",
// 	Short:                 "Startup cluster",
// 	Args:                  cobra.ExactArgs(2),
// 	DisableFlagsInUseLine: true,
// 	Run: func(cmd *cobra.Command, args []string) {
// 		// TODO
// 	},
// }

// var lsConfig = &cobra.Command{
// 	Use:                   "ls",
// 	Short:                 "List all nodes boot configurations",
// 	Args:                  cobra.NoArgs,
// 	DisableFlagsInUseLine: true,
// 	Run: func(cmd *cobra.Command, args []string) {
// 		nodes := internal.GetStaticNodes()

// 		if len(nodes) == 0 {
// 			color.Yellow("No nodes configured\n")
// 			return
// 		}

// 		fmt.Printf("Boot Configurations (%d nodes):\n\n", len(nodes))
// 		for _, node := range nodes {
// 			fmt.Printf("Node: %s\n", color.CyanString(node.Name))
// 			fmt.Printf("  URL: %s\n", node.Url)
// 			fmt.Printf("  Priority (startup): %d\n", node.Boot.StartPriority)
// 			fmt.Printf("  Priority (shutdown): %d\n", node.Boot.ShutdownPriority)
// 			fmt.Printf("  WakeOnLan: %v\n", node.Boot.WoL)
// 			fmt.Println()
// 		}
// 	},
// }

// var shutdownConfig = &cobra.Command{
// 	Use:                   "shutdown",
// 	Short:                 "Shutdown cluster nodes",
// 	Args:                  cobra.NoArgs,
// 	DisableFlagsInUseLine: true,
// 	Run: func(cmd *cobra.Command, args []string) {
// 		client := internal.NewClient()
// 		staticOnly, _ := cmd.Flags().GetBool("static")
// 		all, _ := cmd.Flags().GetBool("all")

// 		if !staticOnly && !all {
// 			color.Red("Error: You must specify either --static or --all\n")
// 			return
// 		}

// 		if staticOnly && all {
// 			color.Red("Error: Cannot specify both --static and --all\n")
// 			return
// 		}

// 		var nodes map[string]config.Node
// 		if staticOnly {
// 			staticNodes := internal.GetStaticNodes()
// 			nodes = make(map[string]config.Node)
// 			for _, node := range staticNodes {
// 				nodes[node.Name] = node
// 			}
// 		} else {
// 			nodes = internal.GetNodes()
// 		}

// 		nodeType := "all cluster"
// 		if staticOnly {
// 			nodeType = "static configured"
// 		}
// 		color.Yellow("WARNING: This will shutdown %s nodes (%d nodes)\n", nodeType, len(nodes))
// 		for _, node := range nodes {
// 			fmt.Printf("  - %s (%s)\n", node.Name, node.Url)
// 		}
// 		fmt.Print("\nAre you sure you want to continue? (yes/no): ")

// 		reader := bufio.NewReader(os.Stdin)
// 		response, err := reader.ReadString('\n')
// 		if err != nil {
// 			color.Red("Error reading input: %v\n", err)
// 			return
// 		}

// 		response = strings.TrimSpace(strings.ToLower(response))
// 		if response != "yes" && response != "y" {
// 			color.Yellow("Shutdown cancelled\n")
// 			return
// 		}

// 		color.Blue("\nShutting down %s nodes\n", nodeType)
// 		for _, node := range nodes {
// 			color.Blue("Shutting down node %s (%s)\n", color.CyanString(node.Name), color.CyanString(node.Url))
// 			if err := client.ShutdownHost(node.Url); err != nil {
// 				color.Red("Error shutting down node %s: %v\n", node.Name, err)
// 				continue
// 			}
// 			color.Green("Shutdown initiated successfully for node %s\n", node.Name)
// 		}
// 	},
// }

// func init() {
// 	shutdownConfig.Flags().Bool("static", false, "Only shutdown static configured nodes")
// 	shutdownConfig.Flags().Bool("all", false, "Shutdown all nodes (static + discovered)")
// 	bootCmd.AddCommand(lsConfig)
// 	bootCmd.AddCommand(startConfig)
// 	bootCmd.AddCommand(shutdownConfig)
// 	rootCmd.AddCommand(bootCmd)
// }
