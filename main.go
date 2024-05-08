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
	eVars := envVarArgs{}
	opts := env.Options{RequiredIfNoDef: true}
	err := env.ParseWithOptions(&eVars, opts)
	if err != nil {
		panic(err.Error())
	}
	// Setup logging
	setLogLevel(eVars.Debug)
	// Setup GitLab connection
	server := gitlab.GraphQlServer{
		GraphQlUrl:  eVars.GitlabQraphqlUrl,
		GitlabToken: eVars.GitlabToken,
		Timeout:     eVars.GitlabTimeoutSecs,
	}
	ValidateCodeownersSyntax(&analysis.Co, server, eVars.ProjectPath, eVars.Branch)
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

func ValidateCodeownersSyntax(pathPrint coPathPrinter, coCheck coChecker, projectPath string, branch string) {
	coPath := pathPrint.CoPath()
	err := coCheck.CheckCodeownersSyntax(coPath, projectPath, branch)
	if err != nil {
		err = fmt.Errorf("CODEOWNERS syntax check resulted in error(s): %w", err)
		panic(err)
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
