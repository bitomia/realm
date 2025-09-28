package main

import (
	"fmt"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/bitomia/realm/cmd/internal"
)

var recipesCmd = &cobra.Command{
	Use:                   "recipes",
	Short:                 "Interface with recipes",
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Realm CLI. Use -h for help.")
	},
}

var launchRecipe = &cobra.Command{
	Use:   "launch [node] [recipe_type]",
	Short: "Launch a pre-configured recipe",
	Long: `Launch a pre-configured recipe. Supported recipe types:
- wordpress_starter, wordpress_pro, wordpress_business
- docker_image
- create_static_project
- delete_static_project`,
	Args:                  cobra.ExactArgs(2),
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		client := internal.NewClient()
		recipeType := args[1]

		// Build recipe data based on type and flags
		recipeData := make(map[string]interface{})
		recipeData["recipe_type"] = recipeType

		// Common flags
		if name, _ := cmd.Flags().GetString("name"); name != "" {
			recipeData["name"] = name
		}
		if domains, _ := cmd.Flags().GetString("domains"); domains != "" {
			domainList := strings.Split(domains, ",")
			for i, domain := range domainList {
				domainList[i] = strings.TrimSpace(domain)
			}
			recipeData["domains"] = domainList
		}

		// Recipe-specific flags
		switch recipeType {
		case "wordpress_starter", "wordpress_pro", "wordpress_business":
			if dbPassword, _ := cmd.Flags().GetString("db-password"); dbPassword != "" {
				recipeData["db_password"] = dbPassword
			}
			if wpPassword, _ := cmd.Flags().GetString("wp-password"); wpPassword != "" {
				recipeData["wp_password"] = wpPassword
			}
		case "docker_image":
			if image, _ := cmd.Flags().GetString("image"); image != "" {
				recipeData["image"] = image
			}
			if port, _ := cmd.Flags().GetInt("port"); port != 0 {
				recipeData["port"] = port
			}
		case "create_static_project":
			if path, _ := cmd.Flags().GetString("path"); path != "" {
				recipeData["path"] = path
			}
		}

		color.Blue("Launching %s recipe on %s\n", color.CyanString(recipeType), color.CyanString(args[0]))
		if err := client.LaunchRecipe(args[0], recipeData); err != nil {
			color.Red("Error launching recipe: %v\n", err)
		} else {
			color.Green("Successfully launched %s recipe\n", color.CyanString(recipeType))
		}
	},
}

var recipeAction = &cobra.Command{
	Use:   "action [node] [recipe_id] [action_type]",
	Short: "Perform actions on existing recipes",
	Long: `Perform actions on existing recipes. Supported action types:
- add_domain: Add domain to static project
- remove_domain: Remove domain from static project`,
	Args:                  cobra.ExactArgs(3),
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		client := internal.NewClient()
		recipeId := args[1]
		actionType := args[2]

		actionData := make(map[string]interface{})
		actionData["action"] = actionType

		// Action-specific parameters
		switch actionType {
		case "add_domain", "remove_domain":
			if domain, _ := cmd.Flags().GetString("domain"); domain != "" {
				actionData["domain"] = domain
			}
		}

		color.Blue("Executing %s action on recipe %s on %s\n", color.CyanString(actionType), color.CyanString(recipeId), color.CyanString(args[0]))
		if err := client.RecipeAction(args[0], recipeId, actionData); err != nil {
			color.Red("Error executing recipe action: %v\n", err)
		} else {
			color.Green("Successfully executed %s action on recipe %s\n", color.CyanString(actionType), color.CyanString(recipeId))
		}
	},
}

var rollbackRecipe = &cobra.Command{
	Use:                   "rollback [node] [recipe_id]",
	Short:                 "Rollback/remove a deployed recipe",
	Args:                  cobra.ExactArgs(2),
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		client := internal.NewClient()
		color.Blue("Rolling back recipe %s on %s\n", color.CyanString(args[1]), color.CyanString(args[0]))
		if err := client.RollbackRecipe(args[0], args[1]); err != nil {
			color.Red("Error rolling back recipe: %v\n", err)
		} else {
			color.Green("Successfully rolled back recipe %s\n", color.CyanString(args[1]))
		}
	},
}

// Helper command to show recipe templates
var recipeTemplates = &cobra.Command{
	Use:                   "templates",
	Short:                 "Show available recipe templates and their parameters",
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		templates := map[string]interface{}{
			"wordpress_starter": map[string]interface{}{
				"description": "WordPress starter deployment",
				"parameters":  []string{"--name", "--domains", "--db-password", "--wp-password"},
			},
			"wordpress_pro": map[string]interface{}{
				"description": "WordPress pro deployment with enhanced resources",
				"parameters":  []string{"--name", "--domains", "--db-password", "--wp-password"},
			},
			"wordpress_business": map[string]interface{}{
				"description": "WordPress business deployment with maximum resources",
				"parameters":  []string{"--name", "--domains", "--db-password", "--wp-password"},
			},
			"docker_image": map[string]interface{}{
				"description": "Generic Docker image deployment",
				"parameters":  []string{"--name", "--domains", "--image", "--port"},
			},
			"create_static_project": map[string]interface{}{
				"description": "Create static website hosting",
				"parameters":  []string{"--name", "--domains", "--path"},
			},
			"delete_static_project": map[string]interface{}{
				"description": "Remove static website hosting",
				"parameters":  []string{"--name"},
			},
		}

		color.Blue("Available Recipe Templates:\n")
		color.Blue("=========================\n")

		for name, info := range templates {
			template := info.(map[string]interface{})
			fmt.Printf("\n%s:\n", color.CyanString(name))
			fmt.Printf("  Description: %s\n", template["description"])
			fmt.Printf("  Parameters: %s\n", color.YellowString(strings.Join(template["parameters"].([]string), ", ")))
		}

		color.Blue("\nUsage Examples:\n")
		color.Blue("===============\n")
		fmt.Println("realm recipes launch http://host:port wordpress_starter --name mysite --domains example.com,www.example.com --db-password secret123 --wp-password admin123")
		fmt.Println("realm recipes launch http://host:port docker_image --name myapp --image nginx:latest --port 80 --domains app.example.com")
		fmt.Println("realm recipes action http://host:port mysite add_domain --domain blog.example.com")
		fmt.Println("realm recipes rollback http://host:port mysite")
	},
}

func init() {
	// Launch command flags
	launchRecipe.Flags().String("name", "", "Recipe name/identifier")
	launchRecipe.Flags().String("domains", "", "Comma-separated list of domains")
	launchRecipe.Flags().String("db-password", "", "Database password (WordPress recipes)")
	launchRecipe.Flags().String("wp-password", "", "WordPress admin password (WordPress recipes)")
	launchRecipe.Flags().String("image", "", "Docker image name (docker_image recipe)")
	launchRecipe.Flags().Int("port", 0, "Container port (docker_image recipe)")
	launchRecipe.Flags().String("path", "", "Static project path (static project recipes)")

	// Action command flags
	recipeAction.Flags().String("domain", "", "Domain name for add/remove domain actions")

	recipesCmd.AddCommand(launchRecipe)
	recipesCmd.AddCommand(recipeAction)
	recipesCmd.AddCommand(rollbackRecipe)
	recipesCmd.AddCommand(recipeTemplates)
	rootCmd.AddCommand(recipesCmd)
}
