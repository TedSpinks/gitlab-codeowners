package main

import (
	"fmt"
	"log/slog"
	"os"
	"slices"
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
	// Check syntax
	server.CheckCodeownersSyntax(analysis.Co.CodeownersFilePath, eVars.ProjectPath, eVars.Branch)
	// Analyze codeowners file structure
	analysis.Co.Analyze()
	fmt.Println("All users and/or groups: ", analysis.Co.UserAndGroupPatterns)
	// Check owner users and groups
	ugLeftovers, err := checkUsersAndGroups(server, analysis.Co.UserAndGroupPatterns)
	if err != nil {
		panic("Error(s) occured while checking users and groups: " + err.Error())
	}
	fmt.Println("Cannot find these users and/or groups: ", ugLeftovers)
	// Check owner emails
	emailleftovers, err := checkEmails(server, analysis.Co.EmailPatterns)
	if err != nil {
		panic("Error(s) occured while checking emails: " + err.Error())
	}
	fmt.Println("Cannot find these emails: ", emailleftovers)
}

func checkEmails(checker emailChecker, emailList []string) (leftovers []string, err error) {
	leftovers = make([]string, len(emailList))
	copy(leftovers, emailList)

	// Check for emails and remove any that are found from the list to check
	emailsFound, err := checker.CheckForUsersByEmail(leftovers)
	if err != nil {
		err = fmt.Errorf("checkEmails() encountered an error while checking emails: %g", err)
		return
	}
	leftovers, err = filterSlice(leftovers, emailsFound)
	if err != nil {
		err = fmt.Errorf("checkEmails() encountered an error while checking emails: %g", err)
		return
	}
	return
}

func checkUsersAndGroups(checker groupUserChecker, combinedList []string) (leftovers []string, err error) {
	leftovers = make([]string, len(combinedList))
	copy(leftovers, combinedList)

	// Check for groups and remove any that are found from the list to check
	groupsFound, err := checker.CheckForGroups(leftovers)
	if err != nil {
		err = fmt.Errorf("checkUsersAndGroups() encountered an error while checking groups: %g", err)
		return
	}
	leftovers, err = filterSlice(leftovers, groupsFound)
	if err != nil {
		err = fmt.Errorf("checkUsersAndGroups() encountered an error while checking groups: %g", err)
		return
	}

	// Check for users and remove any that are found from the list to check
	usersFound, err := checker.CheckForUsers(leftovers)
	if err != nil {
		err = fmt.Errorf("checkUsersAndGroups() encountered an error while checking users: %g", err)
		return
	}
	leftovers, err = filterSlice(leftovers, usersFound)
	if err != nil {
		err = fmt.Errorf("checkUsersAndGroups() encountered an error while checking users: %g", err)
		return
	}

	return
}

// Take the "original" slice and remove all the elements that intersect with the "filterAgainst"
// slice. Returns an error if any elements of "filterAgainst" are missing from "original".
func filterSlice(original []string, filterAgainst []string) ([]string, error) {
	slog.Debug("filterSlice() is filtering original slice: " + strings.Join(original, " "))
	var err error
	for _, filterElement := range filterAgainst {
		originalIndex := slices.IndexFunc(original, func(e string) bool {
			return e == filterElement
		})
		if originalIndex > -1 {
			original = remove(original, originalIndex)
		} else {
			err = fmt.Errorf("filterSlice() - cannot find element '%v' in original slice", filterElement)
			break
		}
	}
	return original, err
}

// Remove the element at index "i" from a slice - without creating a new slice. This is much better for
// performance, but does not preserve the slice's original order. https://stackoverflow.com/a/37335777
func remove(s []string, i int) []string {
	s[i] = s[len(s)-1]
	return s[:len(s)-1]
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
