package main

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/caarlos0/env/v11"
	"gitlab.com/tedspinks/gitlab-codeowners/analysis"
	"gitlab.com/tedspinks/gitlab-codeowners/gitlab"
)

// TO-DO
// Add interfaces to main package

type envVarArgs struct {
	ProjectPath       string `env:"CI_PROJECT_PATH,notEmpty"`
	Branch            string `env:"CI_COMMIT_REF_NAME,notEmpty"`
	GitlabQraphqlUrl  string `env:"CI_API_GRAPHQL_URL,notEmpty"`
	GitlabToken       string `env:"GITLAB_TOKEN,notEmpty"`
	GitlabTimeoutSecs int    `env:"GITLAB_TIMEOUT_SECS" envDefault:"30"`
	Debug             bool   `env:"DEBUG" envDefault:"false"`
	FailNonUserGroups bool   `env:"FAIL_NON_USERS_GROUPS" envDefault:"false"`
}

func main() {
	// Get args from env vars
	envVars := envVarArgs{}
	opts := env.Options{RequiredIfNoDef: true}
	err := env.ParseWithOptions(&envVars, opts)
	if err != nil {
		panic(err.Error())
	}
	// Setup logging
	setLogLevel(envVars.Debug)
	// Setup GitLab connection
	server := gitlab.GraphQlServer{
		GraphQlUrl:  envVars.GitlabQraphqlUrl,
		GitlabToken: envVars.GitlabToken,
		Timeout:     envVars.GitlabTimeoutSecs,
	}
	// Find CODEOWNERS file
	co := analysis.Anatomy{}
	co.DetermineCodeownersPath()
	fmt.Println("CODEOWNERS path: " + co.CodeownersFilePath)
	// Check Users
	userList := []string{"ted-cdw", "tedspinks"}
	usersFound, err := server.CheckForGitLabUsers(userList)
	if err != nil {
		panic("Error(s) occured during user validation: " + err.Error())
	}
	fmt.Println("Users found: " + strings.Join(usersFound, ", "))
	// Check emails
	emailList := []string{"ted.spinks@cdw.com", "gtspinks@hotmail.com"}
	emailsFound, err := server.CheckForGitLabUsersByEmail(emailList)
	if err != nil {
		panic("Error(s) occured during email validation: " + err.Error())
	}
	fmt.Println("Emails found: " + strings.Join(emailsFound, ", "))
	// Check groups
	groupList := []string{"ignw1", "ignw2"}
	groupsFound, err := server.CheckForGroups(groupList)
	if err != nil {
		panic("Error(s) occured during group validation: " + err.Error())
	}
	fmt.Println("Groups found: " + strings.Join(groupsFound, ", "))
	// Check CODEOWNERS
	err = server.CheckCodeownersSyntax("docs/CODEOWNERS", "tedspinks/test-codeowners", "main")
	if err != nil {
		panic("Error(s) occured during syntax validation: " + err.Error())
	}
}

// Set slog's handler to either Info or Debug logging level
func setLogLevel(setToDebug bool) {
	logLevel := slog.LevelInfo
	if setToDebug {
		logLevel = slog.LevelDebug
	}
	opts := &slog.HandlerOptions{
		Level: logLevel,
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, opts))
	slog.SetDefault(logger)
}

type UserChecker interface {
	CheckForGitLabUsers(usernameList []string) (usernamesFound []string)
}
