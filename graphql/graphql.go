package graphql

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	neturl "net/url"
	"strings"
	"time"
)

// Return a list of users and associated emails that are direct members of the specified project. Only returns
// users and emails that the server.GitlabToken identity has permission to see. userSource must be one of:
// DIRECT, INVITED_GROUPS. For self-managed and dedicated SaaS instances of GitLab, I suggest using an admin token.
func (server Server) GetDirectUserMembers(projectFullPath string, userSource string) (usernamesFound []string, emailsFound []string, err error) {
	switch userSource {
	case "DIRECT", "INVITED_GROUPS":
		// valid
	default:
		panic("GetDirectUserMembers() userSource must be one of DIRECT, INVITED_GROUPS: '" + userSource + "'")
	}
	query := `query {project(fullPath: "` + projectFullPath +
		`") {projectMembers(relations: ` + userSource + `) {pageInfo {endCursor startCursor hasNextPage} ` +
		`nodes {id user {id username publicEmail emails {nodes {email}}}}}}}`
	for {
		_, jsonResponse, queryErr := server.RunGraphQlQuery(query)
		if err != nil {
			err = fmt.Errorf("GetDirectUserMembers(): %w", queryErr)
			return
		}
		var queryResults ProjectMembersQueryResponse
		err = json.Unmarshal(jsonResponse, &queryResults)
		if err != nil {
			err = fmt.Errorf("GetDirectUserMembers() error encounted while unmarshaling '%v': %w", string(jsonResponse), err)
			return
		}
		// Append username and any emails to returns
		for _, member := range queryResults.Data.Project.ProjectMembers.Nodes {
			usernamesFound = append(usernamesFound, member.User.Username)
			publicEmail := member.User.PublicEmail
			if publicEmail != "" {
				emailsFound = append(emailsFound, publicEmail)
			}
			for _, email := range member.User.Emails.Nodes {
				if email.Email != publicEmail {
					emailsFound = append(emailsFound, email.Email)
				}
			}
		}
		// Check if the GraphQL results still have another page to process
		if queryResults.Data.Project.ProjectMembers.PageInfo.HasNextPage {
			// Update the query to give the next page of results
			pageEndCursor := queryResults.Data.Project.ProjectMembers.PageInfo.EndCursor
			query = `query {project(fullPath: "` + projectFullPath +
				`") {projectMembers(relations: ` + userSource + ` after:"` + pageEndCursor +
				`") {pageInfo {endCursor startCursor hasNextPage} nodes {id user {id username publicEmail emails {nodes {email}}}}}}}`
		} else {
			// Break if there are no more pages left
			break
		}
	}
	return
}

// Documentation: https://docs.gitlab.com/ee/api/graphql/reference/#repositoryvalidatecodeownerfile
func (server Server) CheckCodeownersSyntax(codeownersPath string, projectPath string, branch string) (err error) {
	// GraphQL search doesn't understand relative paths
	codeownersPath = strings.TrimPrefix(codeownersPath, "./")
	query := `query { project(fullPath: "` + projectPath + `") { repository { validateCodeownerFile(ref: "` + branch +
		`", path: "` + codeownersPath + `") { total validationErrors { code lines }}}}}`
	_, jsonResponse, err := server.RunGraphQlQuery(query)
	if err != nil {
		return fmt.Errorf("CheckCodeownersSyntax() failed: %w", err)
	}
	var queryResults ValidateCodeownersResponse
	err = json.Unmarshal(jsonResponse, &queryResults)
	if err != nil {
		return fmt.Errorf("CheckCodeownersSyntax() could not decode JSON response from GitLab: %w", err)
	}
	if queryResults.Data.Project.Repository.ValidateCodeownerFile == nil {
		return fmt.Errorf("gitlab was unable to find the CODEOWNERS file in project '%v' on branch '%v' at the specified path: '%v'", projectPath, branch, codeownersPath)
	}
	if queryResults.Data.Project.Repository.ValidateCodeownerFile.Total > 0 {
		errorList := []error{}
		for _, validationError := range queryResults.Data.Project.Repository.ValidateCodeownerFile.ValidationErrors {
			errorMessage := validationError.Code
			lines := strings.Trim(strings.Join(strings.Fields(fmt.Sprint(validationError.Lines)), ", "), "[]")
			errorList = append(errorList, fmt.Errorf("validation error '%v' on lines: %v", errorMessage, lines))
		}
		err = errors.Join(errorList...)
	}
	return err
}

// Run the specified query string against the GitLab server's GraphQL API. Returns the API's response as
// a raw (JSON) byte slice, so that the calling function can decode it to its expected type.
func (server Server) RunGraphQlQuery(query string) (statusCode int, responseBody []byte, err error) {
	err = validateUrlWithPath(server.GraphQlUrl)
	if err != nil {
		return
	}
	client := &http.Client{
		Timeout: time.Second * time.Duration(server.Timeout),
	}
	// Encode the qraphqlQuery object as a JSON byte slice
	// We consolidate the query into 1 line so that syntax error messages with a position are easier to pinpoint
	singleLineQuery := consolidateWhitespace(query)
	slog.Debug("Setting up HTTP request for GraphQL query: " + singleLineQuery)
	postData := qraphqlQuery{Query: singleLineQuery}
	postJson, err := json.Marshal(postData)
	if err != nil {
		err = fmt.Errorf("error trying to encode GraphQL query '%v' as JSON: '%w'", query, err)
		return
	}
	// Setup the request
	req, err := http.NewRequest("POST", server.GraphQlUrl, bytes.NewBuffer(postJson))
	// req, err := http.NewRequest("POST", server.GraphQlUrl, strings.NewReader(string(postJson)))  // this also works
	if err != nil {
		err = fmt.Errorf("error trying to create HTTP request to server '%v' with payload '%v': '%w'", server.GraphQlUrl, query, err)
		return
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", "Bearer "+server.GitlabToken)
	// Make the request
	slog.Debug("Making HTTP request:", slog.Any("httpRequest", req))
	res, err := client.Do(req)
	if err != nil {
		err = fmt.Errorf("error making HTTP request to server '%v' with payload '%v': '%w'", server.GraphQlUrl, query, err)
		return
	}
	// Return the results
	statusCode = res.StatusCode
	defer res.Body.Close()
	responseBody, err = io.ReadAll(res.Body)
	if err != nil {
		err = fmt.Errorf("error reading response from server '%v' with GraphQL query '%v': '%w'", server.GraphQlUrl, query, err)
		return
	}
	slog.Debug("HTTP response received:", slog.Any(fmt.Sprint(res.StatusCode), responseBody))
	if res.StatusCode != http.StatusOK {
		err = fmt.Errorf("graphQL request to server '%v' with query '%v' returned status %d", server.GraphQlUrl, query, res.StatusCode)
	}
	err = getGraphQlErrors(responseBody)
	if err != nil {
		err = fmt.Errorf("graphQL query '%v' received status code %d and errors: %w", query, res.StatusCode, err)
		return
	}
	return
}

// Check the JSON byte slice from the GraphQL response for errors, and return them as an error.
// Also print any errors to the debug log. Example of an error that was returned with an HTTP status 200:
// {"errors":[{"message":"Expected NAME, actual: LBRACKET (\"[\") at [1, 135]","locations":[{"line":1,"column":135}]}]}
func getGraphQlErrors(jsonResponse []byte) (err error) {
	var queryErrors QueryErrors
	err = json.Unmarshal(jsonResponse, &queryErrors)
	switch {
	case err != nil:
		return err
	case len(queryErrors.Errors) > 0:
		errorsToJoin := []error{}
		for _, queryError := range queryErrors.Errors {
			errorsToJoin = append(errorsToJoin, errors.New(queryError.Message))
		}
		err = errors.Join(errorsToJoin...)
	}
	return err
}

// Return an error if the provided URL is not valid
func validateUrlWithPath(url string) (err error) {
	u, err := neturl.Parse(url)
	if err != nil {
		err = fmt.Errorf("cannot parse URL '%v': %w", url, err)
		return
	}
	if u.Scheme == "" || u.Host == "" {
		err = fmt.Errorf("invalid URL '%v'", url)
		return
	}
	if u.Path == "" {
		err = fmt.Errorf("URL does not contain a path '%v'", url)
		return
	}
	return
}

// Replace consecutive occurences of whitespace characters with a single space
func consolidateWhitespace(s string) string {
	// strings.Fields() splits on any amount of white space
	return strings.Join(strings.Fields(strings.TrimSpace(s)), " ")
}
