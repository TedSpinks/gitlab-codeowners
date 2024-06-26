package main

import (
	"fmt"
	"log/slog"
	"os"
	"slices"
	"strings"

	"github.com/bmatcuk/doublestar" // because Glob() in "path/filepath" doesn't support "**"
	"github.com/caarlos0/env/v11"
	"gitlab.com/tedspinks/validate-codeowners/analysis"
	"gitlab.com/tedspinks/validate-codeowners/graphql"
	"gitlab.com/tedspinks/validate-codeowners/rest"
)

type envVarArgs struct {
	ProjectPath       string `env:"CI_PROJECT_PATH,notEmpty"`
	Branch            string `env:"CI_COMMIT_REF_NAME,notEmpty"`
	GitlabGraphqlUrl  string `env:"CI_API_GRAPHQL_URL,notEmpty"`
	GitlabRestUrl     string `env:"CI_API_V4_URL,notEmpty"`
	GitlabToken       string `env:"GITLAB_TOKEN,notEmpty"`
	GitlabTimeoutSecs int    `env:"GITLAB_TIMEOUT_SECS" envDefault:"30"`
	Debug             bool   `env:"CODEOWNERS_DEBUG" envDefault:"false"`
}

func main() {
	// Get args from env vars
	eVars := envVarArgs{}
	getEnvVerArgs(&eVars)
	// Prep
	setLogLevel(eVars.Debug)
	graphqlServer, restServer := setupGitlabConnections(eVars)
	hasFailures := false
	// Make sure codeowners syntax is valid before trying to analyze it
	checkSyntax(graphqlServer, analysis.Co.CodeownersFilePath, eVars.ProjectPath, eVars.Branch)
	// Analyze codeowners file structure
	analysis.Co.Analyze()
	if !checkAndPrintResults("Malformed users and groups check", nil, analysis.Co.IgnoredPatterns, "Users or groups that do not start with '@':") {
		hasFailures = true
	}
	// Check owners
	ugList := analysis.Co.UserAndGroupPatterns
	eList := analysis.Co.EmailPatterns
	userAndGroupLeftovers, emailLeftovers, err := checkOwners(graphqlServer, restServer, eVars.ProjectPath, ugList, eList)
	if !checkAndPrintResults("Direct user and group membership check", err, userAndGroupLeftovers, "Unable to find:") {
		hasFailures = true
	}
	if !checkAndPrintResults("Direct user email membership check", err, emailLeftovers, "Unable to find:") {
		hasFailures = true
	}
	// Check file patterns
	badFilePatterns, err := checkFilePatterns(analysis.Co.FilePatterns)
	if !checkAndPrintResults("File pattern check", err, badFilePatterns, "Unable to find:") {
		hasFailures = true
	}
	// Exit
	if hasFailures {
		fmt.Println("\nSee failures noted above.")
		os.Exit(1)
	}
}

// Read in the program args from environment variables. Stop the program if there are any errors.
func getEnvVerArgs(eVars *envVarArgs) {
	opts := env.Options{RequiredIfNoDef: true}
	err := env.ParseWithOptions(eVars, opts)
	if err != nil {
		fmt.Println("\nError " + err.Error())
		os.Exit(1)
	}
}

// Check codeowners syntax. Stop the program if there are syntax errors, since there's no sense in trying to
// analyze a broken file.
func checkSyntax(checker syntaxChecker, coFilePath string, projectPath string, branch string) {
	err := checker.CheckCodeownersSyntax(coFilePath, projectPath, branch)
	if err != nil {
		fmt.Println("\nSyntax check of CODEOWNERS: FAILED")
		fmt.Println(err.Error())
		os.Exit(1)
	}
	fmt.Printf("\nSyntax check of '%v': PASSED\n", analysis.Co.CodeownersFilePath)
}

// Setup GitLab connections - return struct vars with connection info for both of the GitLab API packages
func setupGitlabConnections(eVars envVarArgs) (graphql.Server, rest.Server) {
	graphqlServer := graphql.Server{
		GraphQlUrl:  eVars.GitlabGraphqlUrl,
		GitlabToken: eVars.GitlabToken,
		Timeout:     eVars.GitlabTimeoutSecs,
	}
	restServer := rest.Server{
		RestUrl:     eVars.GitlabRestUrl,
		GitlabToken: eVars.GitlabToken,
		Timeout:     eVars.GitlabTimeoutSecs,
	}
	return graphqlServer, restServer
}

// Returns true if the results of a check indicate a pass (no error and leftovers is empty).
// Returns false for failure(s). Prints the failure details to the console for the user to read.
func checkAndPrintResults(checkName string, err error, leftovers []string, leftoverMsg string) (passed bool) {
	passed = (len(leftovers) == 0 && err == nil)
	status := "PASSED"
	if !passed {
		status = "FAILED"
	}
	fmt.Println("\n" + checkName + ": " + status)
	indent := "     "
	if err != nil {
		fmt.Println(indent + "error: " + err.Error())
	} else if !passed {
		fmt.Println(indent + leftoverMsg)
		for _, leftover := range leftovers {
			fmt.Println(indent + indent + leftover)
		}
	}
	return
}

// Verify that each file pattern matches at least one file. Return any patterns that do not have any matches.
func checkFilePatterns(filePatterns []string) (badPatterns []string, err error) {
	for _, pattern := range filePatterns {
		slog.Debug("checkFilePatterns(): Checking file pattern '" + pattern + "'")
		if pattern == "*" { // No need to check this pattern, as it will always have at least one match (the CODEOWNERS file)
			continue
		}
		globExpression := translateCoToGlob(pattern)
		slog.Debug("checkFilePatterns(): translated to glob expression '" + globExpression + "'")
		matches, matchErr := doublestar.Glob(globExpression)
		if matchErr != nil {
			err = fmt.Errorf("checkFilePatterns() error while evaluating glob '%v': %w", pattern, matchErr)
			return
		}
		slog.Debug(fmt.Sprintf("checkFilePatterns(): found %d matches for glob expression '%v'", len(matches), globExpression))
		if len(matches) == 0 {
			badPatterns = append(badPatterns, pattern)
		}
	}
	return
}

// Translate a CODEOWNERS file pattern into a standard glob expression.
func translateCoToGlob(pattern string) (translatedPattern string) {
	translatedPattern = pattern
	if strings.HasPrefix(pattern, "/") {
		// https://docs.gitlab.com/ee/user/project/codeowners/reference.html#absolute-paths
		translatedPattern = "." + translatedPattern
	} else {
		// https://docs.gitlab.com/ee/user/project/codeowners/reference.html#relative-paths
		translatedPattern = "./**/" + translatedPattern
	}
	if strings.HasSuffix(pattern, "/") {
		// https://docs.gitlab.com/ee/user/project/codeowners/reference.html#directory-paths
		translatedPattern = translatedPattern + "**/*"
	}
	return
}

// Check that owner entries (users, groups, emails) are direct members of the project. Since user and group owners are both
// specified by "@name" and are therefore indistinguishable until checked, these are provided in a combined list.
// Returns any remaining users/groups and emails that were not found as direct members of the project.
func checkOwners(uChecker userChecker, gChecker groupChecker, projectFullPath string, ugList []string, emailList []string) (
	remainingUsersGroups []string,
	remainingEmails []string,
	err error,
) {
	// Make editable copies of the lists, so that we can remove items as we verify them (i.e. check them off the list)
	remainingUsersGroups = make([]string, len(ugList))
	copy(remainingUsersGroups, ugList)
	remainingEmails = make([]string, len(emailList))
	copy(remainingEmails, emailList)

	slog.Debug("checkOwners() is checking off groups that are direct members of the project...")
	groupsFound, err := gChecker.GetDirectGroupMembers(projectFullPath)
	if err != nil {
		err = fmt.Errorf("checkOffUsersAndGroups() errored in gChecker.GetDirectGroupMembers(): %w", err)
		return
	}
	remainingUsersGroups = filterSlice(remainingUsersGroups, groupsFound)
	if len(remainingUsersGroups) == 0 && len(remainingEmails) == 0 { // All checked off?
		return
	}

	slog.Debug("checkOwners() is checking off users+emails in groups that are direct members of the project...")
	usernamesFound, emailsFound, err := uChecker.GetDirectUserMembers(projectFullPath, "INVITED_GROUPS")
	if err != nil {
		err = fmt.Errorf("checkOffUsersAndGroups() errored in uChecker.GetDirectUserMembers() INVITED_GROUPS: %w", err)
		return
	}
	remainingUsersGroups = filterSlice(remainingUsersGroups, usernamesFound)
	remainingEmails = filterSlice(remainingEmails, emailsFound)
	if len(remainingUsersGroups) == 0 && len(remainingEmails) == 0 { // All checked off?
		return
	}

	slog.Debug("checkOwners() is checking off users+emails that are themselves direct members of the project...")
	usernamesFound, emailsFound, err = uChecker.GetDirectUserMembers(projectFullPath, "DIRECT")
	if err != nil {
		err = fmt.Errorf("checkOffUsersAndGroups() errored in uChecker.GetDirectUserMembers() DIRECT: %w", err)
		return
	}
	remainingUsersGroups = filterSlice(remainingUsersGroups, usernamesFound)
	remainingEmails = filterSlice(remainingEmails, emailsFound)
	return
}

// Take the "original" slice and remove all the elements that intersect with the "filterAgainst"
// slice. Return the new slice.
func filterSlice(original []string, filterAgainst []string) (filteredList []string) {
	slog.Debug("filterSlice() is filtering original slice: " + strings.Join(original, " "))
	// Max size of the filtered output list is the original list size (if no elements intersect)
	filteredList = make([]string, 0, len(original))
	// Check each element of the original list against the filterAgainst list
	for _, originalElement := range original {
		intersect := slices.IndexFunc(filterAgainst, func(e string) bool {
			return e == originalElement
		})
		// If this element is not in filterAgainst, then keep it
		if intersect == -1 {
			filteredList = append(filteredList, originalElement)
		}
	}
	return
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
