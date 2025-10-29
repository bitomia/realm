package handlers

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/mux"

	"github.com/bitomia/realm/daemon/proxy"
	"github.com/bitomia/realm/daemon/recipes"
	"github.com/bitomia/realm/daemon/utils"
)

type LaunchRecipeOpts struct {
	Recipe string          `json:"recipe"`
	Args   json.RawMessage `json:"args,omitempty"`
}

// NOTE this will be relayed to the browser throught redux-backend
// remember to delete secret information before sending this to browser
type Recipe struct {
	RecipeId  uuid.UUID `json:"recipe_id"`
	SSOSecret string    `json:"sso_secret,omitempty"`
}

type RecipeError struct {
	Error string `json:"error"`
}

type WordpressPlan int

const (
	WordpressStarter WordpressPlan = iota
	WordpressPro
	WordpressBusiness
)

func launchWordpressHandler(w http.ResponseWriter, recipeId uuid.UUID, launchOpts LaunchRecipeOpts, plan WordpressPlan) {
	var opts recipes.WordpressRecipeOpts
	json.Unmarshal([]byte(launchOpts.Args), &opts)

	if opts.Validate() == false {
		utils.HttpError(w, http.StatusBadRequest, "Invalid options")
		return
	}

	var planOpts recipes.WordpressPlanOpts
	switch plan {
	case WordpressStarter:
		planOpts = recipes.WordpressPlanOpts{
			WPVolumeSize: "15GB",
			WPMemLimit:   512,
			DBVolumeSize: "15GB",
			DBMemLimit:   128,
		}
	case WordpressPro:
		planOpts = recipes.WordpressPlanOpts{
			WPVolumeSize: "25GB",
			WPMemLimit:   512,
			DBVolumeSize: "25GB",
			DBMemLimit:   256,
		}
	case WordpressBusiness:
		planOpts = recipes.WordpressPlanOpts{
			WPVolumeSize: "40GB",
			WPMemLimit:   512,
			DBVolumeSize: "40GB",
			DBMemLimit:   512,
		}
	default:
		utils.HttpError(w, http.StatusBadRequest, "Invalid plan")
		return
	}
	recipeRet, err := recipes.LaunchWordpress(w, recipeId, opts, planOpts)
	if err == nil {
		json.NewEncoder(w).Encode(Recipe{recipeId, recipeRet.SSOSecret})
	} else {
		errorMsg := fmt.Sprintf("wordpress recipe failed %s: %v", recipeId, err)
		slog.Error("LaunchWordpress", "error", errorMsg)
		json.NewEncoder(w).Encode(RecipeError{errorMsg})
	}
}

func launchDockerImageHandler(w http.ResponseWriter, recipeId uuid.UUID, launchOpts LaunchRecipeOpts) {
	var opts recipes.DockerImageRecipeOpts
	json.Unmarshal([]byte(launchOpts.Args), &opts)

	_, err := recipes.LaunchDockerImage(w, recipeId, opts, 256)
	if err == nil {
		json.NewEncoder(w).Encode(Recipe{recipeId, ""})
	} else {
		errorMsg := fmt.Sprintf("docker recipe failed %s: %v", recipeId, err)
		slog.Error("LaunchDocker", "error", errorMsg)
		json.NewEncoder(w).Encode(RecipeError{errorMsg})
	}
}

type CreateStaticProjectRecipeOpts struct {
	Domain    string `json:"domain"`
	ProjectID string `json:"project_id"`
}

func launchCreateStaticProject(w http.ResponseWriter, launchOpts LaunchRecipeOpts) {
	var opts CreateStaticProjectRecipeOpts
	json.Unmarshal([]byte(launchOpts.Args), &opts)

	if len(opts.Domain) == 0 {
		utils.HttpError(w, http.StatusBadRequest, "create_static_project recipe failed. Domain cannot be empty")
		return
	}
	if len(opts.ProjectID) == 0 {
		utils.HttpError(w, http.StatusBadRequest, "create_static_project recipe failed. ProjectID cannot be empty")
		return
	}

	err := proxy.CreateStaticProject(opts.ProjectID, opts.Domain)
	if err != nil {
		utils.HttpError(w, http.StatusBadRequest, "create_static_project recipe failed %v: %v", opts, err)
		return
	}
}

type StaticDomainOpts struct {
	Domain    string `json:"domain"`
	ProjectID string `json:"project_id"`
}

func launchAddStaticDomain(w http.ResponseWriter, launchOpts LaunchRecipeOpts) {
	var opts StaticDomainOpts
	json.Unmarshal([]byte(launchOpts.Args), &opts)

	if len(opts.Domain) == 0 {
		utils.HttpError(w, http.StatusBadRequest, "add_static_domain recipe failed. Domain cannot be empty")
		return
	}
	if len(opts.ProjectID) == 0 {
		utils.HttpError(w, http.StatusBadRequest, "add_static_domain recipe failed. ProjectID cannot be empty")
		return
	}

	err := proxy.AddStaticDomain(opts.ProjectID, opts.Domain)
	if err != nil {
		utils.HttpError(w, http.StatusBadRequest, "add_static_domain recipe failed %v: %v", opts, err)
		return
	}
}

func launchRemoveStaticDomain(w http.ResponseWriter, launchOpts LaunchRecipeOpts) {
	var opts StaticDomainOpts
	json.Unmarshal([]byte(launchOpts.Args), &opts)

	slog.Info("launchremoveStaticDomain", "projectID", opts.ProjectID, "domain", opts.Domain)

	if len(opts.Domain) == 0 {
		utils.HttpError(w, http.StatusBadRequest, "remove_static_domain recipe failed. Domain cannot be empty")
		return
	}
	if len(opts.ProjectID) == 0 {
		utils.HttpError(w, http.StatusBadRequest, "remove_static_domain recipe failed. ProjectID cannot be empty")
		return
	}

	err := proxy.RemoveStaticDomain(opts.ProjectID, opts.Domain)
	if err != nil {
		utils.HttpError(w, http.StatusBadRequest, "remove_static_domain recipe failed %v: %v", opts, err)
		return
	}
}

type DeleteStaticProjectRecipeOpts struct {
	ProjectID string `json:"project_id"`
}

func launchDeleteStaticProject(w http.ResponseWriter, launchOpts LaunchRecipeOpts) {
	var opts DeleteStaticProjectRecipeOpts
	json.Unmarshal([]byte(launchOpts.Args), &opts)

	if len(opts.ProjectID) == 0 {
		utils.HttpError(w, http.StatusBadRequest, "delete_static_domain recipe failed. Domain cannot be empty")
		return
	}

	err := proxy.DeleteStaticProject(opts.ProjectID)
	if err != nil {
		utils.HttpError(w, http.StatusBadRequest, "delete_static_domain recipe failed %v: %v", opts, err)
		return
	}
}

func LaunchRecipeHandler(w http.ResponseWriter, r *http.Request) {
	recipeId := uuid.New()
	slog.Info("recipes.LaunchHandler", "recipeID", recipeId)

	var opts LaunchRecipeOpts
	json.NewDecoder(r.Body).Decode(&opts)

	slog.Info("recipes.LaunchHandler", "recipeID", recipeId, "recipe", opts.Recipe)

	switch opts.Recipe {
	case "add_static_domain":
		launchAddStaticDomain(w, opts)
	case "delete_static_domain":
		launchRemoveStaticDomain(w, opts)
	case "create_static_project":
		launchCreateStaticProject(w, opts)
	case "delete_static_project":
		launchDeleteStaticProject(w, opts)
	case "wordpress_starter":
		launchWordpressHandler(w, recipeId, opts, WordpressStarter)
	case "wordpress_pro":
		launchWordpressHandler(w, recipeId, opts, WordpressPro)
	case "wordpress_business":
		launchWordpressHandler(w, recipeId, opts, WordpressBusiness)
	case "docker_image":
		launchDockerImageHandler(w, recipeId, opts)
	default:
		utils.HttpError(w, http.StatusBadRequest, "Unknown recipe")
	}
}

type RollbackRecipeOpts struct {
	Recipe string `json:"recipe"`
}

func RollbackHandler(w http.ResponseWriter, r *http.Request) {
	recipeId := mux.Vars(r)["recipeId"]
	slog.Info("recipes.RollbackHandler", "recipeID", recipeId)

	var opts RollbackRecipeOpts
	json.NewDecoder(r.Body).Decode(&opts)

	switch opts.Recipe {
	case "wordpress_starter", "wordpress_pro", "wordpress_business":
		err := recipes.RollbackWordpress(recipeId)
		if err != nil {
			utils.HttpError(w, http.StatusBadRequest, "wordpress_starter recipe failed %s: %v", recipeId, err)
			return
		}
		json.NewEncoder(w).Encode(recipeId)
		return
	case "docker_image":
		err := recipes.RollbackDockerImage(recipeId)
		if err != nil {
			utils.HttpError(w, http.StatusBadRequest, "docker_image recipe failed %s: %v", recipeId, err)
			return
		}
		json.NewEncoder(w).Encode(recipeId)
		return
	default:
		utils.HttpError(w, http.StatusBadRequest, "Unknown recipe")
		return
	}
}

type RecipeActionOpts struct {
	Recipe string          `json:"recipe"`
	Args   json.RawMessage `json:"args,omitempty"`
}

type WordpressDomainOpts struct {
	Domain string `json:"domain"`
}

func RecipeActionHandler(w http.ResponseWriter, r *http.Request) {
	recipeId := mux.Vars(r)["recipeId"]
	var actionOpts RecipeActionOpts
	json.NewDecoder(r.Body).Decode(&actionOpts)

	slog.Info("recipes.RecipeUpdateHandler", "recipeID", recipeId, "recipe", actionOpts.Recipe)

	switch actionOpts.Recipe {
	case "add_wordpress_domain":
		var opts WordpressDomainOpts
		json.Unmarshal([]byte(actionOpts.Args), &opts)

		if len(opts.Domain) == 0 {
			utils.HttpError(w, http.StatusBadRequest, "add_wordpress_domain recipe failed. Domain cannot be empty: %s", recipeId)
			return
		}

		err := recipes.AddWordpressDomain(recipeId, opts.Domain)
		if err != nil {
			utils.HttpError(w, http.StatusBadRequest, "add_wordpress_domain recipe failed %s: %v", recipeId, err)
			return
		}
		json.NewEncoder(w).Encode(recipeId)
		return
	case "remove_wordpress_domain":
		var opts WordpressDomainOpts
		json.Unmarshal([]byte(actionOpts.Args), &opts)

		if len(opts.Domain) == 0 {
			utils.HttpError(w, http.StatusBadRequest, "remove_wordpress_domain recipe failed. Domain cannot be empty: %s", recipeId)
			return
		}

		err := recipes.RemoveWordpressDomain(recipeId, opts.Domain)
		if err != nil {
			utils.HttpError(w, http.StatusBadRequest, "remove_wordpress_domain recipe failed %s: %v", recipeId, err)
			return
		}
		json.NewEncoder(w).Encode(recipeId)
		return
	default:
		utils.HttpError(w, http.StatusBadRequest, "Unknown recipe")
		return
	}
}
