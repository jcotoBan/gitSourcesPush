package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"gopkg.in/yaml.v2"
)

// Config struct to match the config.yaml file
type Config struct {
	Config struct {
		SecureURL           string   `yaml:"secure_url"`
		SecureAPIToken      string   `yaml:"secure_api_token"`
		GithubToken         string   `yaml:"github_token"`
		AccountType         string   `yaml:"accountType"`
		AccountName         string   `yaml:"accountName"`
		IntegrationID       string   `yaml:"integrationId"`
		PRScanBranchPattern string   `yaml:"prScanBranchPattern"`
		Folders             []string `yaml:"folders"`
	} `yaml:"config"`
}

// Repository struct for GitHub API response
type Repository struct {
	Name string `json:"name"`
}

// LoadConfig reads and parses the YAML configuration file
func LoadConfig(filename string) (*Config, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var config Config
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}

// Fetch GitHub repositories based on account type
func getGitHubRepositories(githubToken, accountType, accountName string) ([]string, error) {
	var url string
	if accountType == "user" {
		url = "https://api.github.com/user/repos"
	} else if accountType == "org" {
		url = fmt.Sprintf("https://api.github.com/orgs/%s/repos", accountName)
	} else {
		return nil, fmt.Errorf("invalid account type: must be 'user' or 'org'")
	}

	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	// Set authentication and headers
	req.Header.Set("Authorization", "token "+githubToken)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return nil, fmt.Errorf("GitHub API request failed: %s", body)
	}

	// Parse JSON response
	var repos []Repository
	err = json.NewDecoder(resp.Body).Decode(&repos)
	if err != nil {
		return nil, err
	}

	// Extract repository names
	var repoNames []string
	for _, repo := range repos {
		repoNames = append(repoNames, repo.Name)
	}

	return repoNames, nil
}

func main() {
	// Load configuration from config.yaml
	config, err := LoadConfig("config.yaml")
	if err != nil {
		fmt.Println("Error loading configuration:", err)
		return
	}

	githubToken := config.Config.GithubToken
	accountType := config.Config.AccountType
	accountName := config.Config.AccountName
	sysdigURL := fmt.Sprintf("%s/api/cspm/v1/gitProvider/gitSources", strings.TrimRight(config.Config.SecureURL, "/"))
	apiToken := config.Config.SecureAPIToken
	integrationID := config.Config.IntegrationID
	prScanBranchPattern := config.Config.PRScanBranchPattern
	folders := config.Config.Folders

	// Fetch repositories from GitHub
	repositories, err := getGitHubRepositories(githubToken, accountType, accountName)
	if err != nil {
		fmt.Println("Error fetching repositories:", err)
		return
	}

	client := &http.Client{}

	for _, repo := range repositories {
		data := map[string]interface{}{
			"source": map[string]interface{}{
				"repository":          repo,
				"folders":             folders,
				"prScanBranchPattern": prScanBranchPattern,
				"integrationId":       integrationID,
				"name":                fmt.Sprintf("%s_source", repo),
			},
		}

		jsonData, err := json.Marshal(data)
		if err != nil {
			fmt.Printf("Failed to marshal JSON for %s: %v\n", repo, err)
			continue
		}

		req, err := http.NewRequest("POST", sysdigURL, bytes.NewBuffer(jsonData))
		if err != nil {
			fmt.Printf("Failed to create request for %s: %v\n", repo, err)
			continue
		}

		req.Header.Set("Authorization", "Bearer "+apiToken)
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			fmt.Printf("Request failed for %s: %v\n", repo, err)
			continue
		}
		defer resp.Body.Close()

		body, _ := ioutil.ReadAll(resp.Body)
		if resp.StatusCode == http.StatusOK {
			fmt.Printf("Successfully added %s\n", repo)
		} else {
			fmt.Printf("Failed to add %s: %s\n", repo, body)
		}
	}
}
