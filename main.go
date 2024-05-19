package main

import (
	"fmt"
	"log"
	"log/slog"
	"os"
	"slices"
	"strings"

	filepath "github.com/bmatcuk/doublestar" // because Glob() in "path/filepath" doesn't support "**"
	"github.com/caarlos0/env/v11"
	"gitlab.com/tedspinks/gitlab-codeowners/analysis"
	"gitlab.com/tedspinks/gitlab-codeowners/graphql"
	"gitlab.com/tedspinks/gitlab-codeowners/rest"
)

type envVarArgs struct {
	ProjectPath       string `env:"CI_PROJECT_PATH,notEmpty"`
	Branch            string `env:"CI_COMMIT_REF_NAME,notEmpty"`
	GitlabGraphqlUrl  string `env:"CI_API_GRAPHQL_URL,notEmpty"`
	GitlabRestUrl     string `env:"CI_API_V4_URL,notEmpty"`
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
		log.Fatalln("error reading environment variables: " + err.Error())
	}
	// Setup logging
	setLogLevel(eVars.Debug)
	// Setup GitLab connection
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
	// Check codeowners syntax
	err = graphqlServer.CheckCodeownersSyntax(analysis.Co.CodeownersFilePath, eVars.ProjectPath, eVars.Branch)
	if err != nil {
		log.Fatalln("error validating CODEOWNERS syntax: " + err.Error())
	}
	// Analyze codeowners file structure
	analysis.Co.Analyze()
	fmt.Println("All user and/or group patterns: ", analysis.Co.UserAndGroupPatterns)
	fmt.Println("All file patterns: ", analysis.Co.FilePatterns)
	var failedChecks []string
	// Check owners
	ugList := analysis.Co.UserAndGroupPatterns
	eList := analysis.Co.EmailPatterns
	userAndGroupLeftovers, emailLeftovers, err := checkOwners(graphqlServer, restServer, eVars.ProjectPath, ugList, eList)
	if err != nil {
		panic("error while checking owner patterns: " + err.Error())
	}
	if len(userAndGroupLeftovers) > 0 {
		msg := "Did not find these users and/or groups as direct project members: " + strings.Join(userAndGroupLeftovers, ", ")
		failedChecks = append(failedChecks, msg)
	}
	if len(emailLeftovers) > 0 {
		msg := "Did not find these emails as direct project members: " + strings.Join(emailLeftovers, ", ")
		failedChecks = append(failedChecks, msg)
	}
	// Check file patterns
	badPatterns, err := checkFilePatterns(analysis.Co.FilePatterns)
	if err != nil {
		panic("error while checking file patterns: " + err.Error())
	}
	if len(badPatterns) > 0 {
		msg := "The following file patterns did not evaluate to files in the project: " + strings.Join(badPatterns, ", ")
		failedChecks = append(failedChecks, msg)
	}
	// Print results and exit
	handleFailures(failedChecks)
}

func handleFailures(failedChecks []string) {
	if len(failedChecks) > 0 {
		for _, failure := range failedChecks {
			fmt.Fprintln(os.Stderr, failure)
		}
		log.Fatal("Failures noted above.")
	}
}

func checkFilePatterns(filePatterns []string) (badPatterns []string, err error) {
	for _, pattern := range filePatterns {
		slog.Debug("checkFilePatterns(): Checking file pattern '" + pattern + "'")
		if pattern == "*" { // No need to check this pattern, as it will always have at least one match (the CODEOWNERS file)
			continue
		}
		globExpression := translateCoToGlob(pattern)
		slog.Debug("checkFilePatterns(): translated to glob expression '" + globExpression + "'")
		matches, matchErr := filepath.Glob(globExpression)
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

// Translate a CODEOWNERS file pattern into a standard glob expression
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

// Checks that owner entries (users, groups, emails) are direct members of the project. Since user and group owners are both
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

func checkEmails(checker emailChecker, emailList []string) (leftovers []string, err error) {
	leftovers = make([]string, len(emailList))
	copy(leftovers, emailList)

	// Check for emails and remove any that are found from the list to check
	emailsFound, err := checker.CheckForUsersByEmail(leftovers)
	if err != nil {
		err = fmt.Errorf("checkEmails() encountered an error while checking emails: %g", err)
		return
	}
	leftovers = filterSlice(leftovers, emailsFound)
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
	leftovers = filterSlice(leftovers, groupsFound)

	// Check for users and remove any that are found from the list to check
	usersFound, err := checker.CheckForUsers(leftovers)
	if err != nil {
		err = fmt.Errorf("checkUsersAndGroups() encountered an error while checking users: %g", err)
		return
	}
	leftovers = filterSlice(leftovers, usersFound)

	return
}

// Take the "original" slice and remove all the elements that intersect with the "filterAgainst"
// slice.
func filterSlice(original []string, filterAgainst []string) []string {
	slog.Debug("filterSlice() is filtering original slice: " + strings.Join(original, " "))
	for _, filterElement := range filterAgainst {
		slog.Debug("...filtering '" + filterElement + "'")
		originalIndex := slices.IndexFunc(original, func(e string) bool {
			return e == filterElement
		})
		if originalIndex > -1 {
			original = remove(original, originalIndex)
		}
	}
	return original
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
