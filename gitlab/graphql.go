package gitlab

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"
)

// Search in GitLab for the specified list of potential usernames. Return a list of all of the
// usernames that were found as valid users in GitLab.
// Search documentation: https://docs.gitlab.com/ee/api/graphql/users_example.html
func (server GraphQlServer) CheckForGitLabUsers(usernameList []string) (usernamesFound []string, err error) {
	// Create the GraphQL query string
	usernameListString := ""
	for i := 0; i < len(usernameList); i++ {
		usernameListString += `"` + usernameList[i] + `"`
		if i+1 < len(usernameList) {
			usernameListString += ","
		}
	}
	query := `query { users(usernames: [` + usernameListString + `]) { pageInfo { endCursor startCursor hasNextPage } nodes { id username publicEmail emails { nodes { email } }}}}`
	// Searches for lots of users could result in multiple pages of results (100/page), so
	// repeat the GraphQL query until all the pages have been retrieved
	for {
		_, jsonResponse, err := server.RunGraphQlQuery(query)
		if err != nil {
			return usernamesFound, err
		}
		var queryResults UserQueryResponse
		err = json.Unmarshal(jsonResponse, &queryResults)
		if err != nil {
			return usernamesFound, err
		}
		for _, user := range queryResults.Data.Users.Nodes {
			usernamesFound = append(usernamesFound, user.Username)
		}
		// Check if the GraphQL results still have another page to process
		if queryResults.Data.Users.PageInfo.HasNextPage {
			// Update the query to give the next page of results
			pageEndCursor := queryResults.Data.Users.PageInfo.EndCursor
			query = `query { users(usernames: [` + usernameListString + `] after:"` + pageEndCursor + `") { pageInfo { endCursor startCursor hasNextPage } nodes { id username publicEmail emails { nodes { email } }}}}`
		} else {
			// Break if there are no more pages left
			break
		}
	}
	return usernamesFound, err
}

// Search in GitLab for the specified list of email addresses. Return a list of all of the
// email addresses that were found as valid users in GitLab.
// IMPORTANT: you must use an Admin GitLab token in order for the search to find users' emails!
// Otherwise, the seach results will be error-free, but will only contain emails of users
// who have enabled a "Public email" in their GitLab user settings.
// Search documentation: https://docs.gitlab.com/ee/api/graphql/reference/#queryusers
func (server GraphQlServer) CheckForGitLabUsersByEmail(emailList []string) (emailsFound []string, err error) {
	for _, email := range emailList {
		query := `query { users(search: "` + email + `") { pageInfo { endCursor startCursor hasNextPage } nodes { id username publicEmail emails { nodes {email} } }}}`
		_, jsonResponse, err := server.RunGraphQlQuery(query)
		if err != nil {
			wrappedErr := fmt.Errorf("CheckForGitLabUsersByEmail() failed on email '%v': %w", email, err)
			return emailsFound, wrappedErr
		}
		var queryResults UserQueryResponse
		err = json.Unmarshal(jsonResponse, &queryResults)
		if err != nil {
			wrappedErr := fmt.Errorf("CheckForGitLabUsersByEmail() could not decode JSON response from search for email '%v': %w", email, err)
			slog.Debug("JSON response: " + string(jsonResponse))
			return emailsFound, wrappedErr
		}
		for _, user := range queryResults.Data.Users.Nodes {
			if (user.PublicEmail != "") && (!slices.Contains(emailsFound, user.PublicEmail)) {
				emailsFound = append(emailsFound, user.PublicEmail)
			}
			for _, privateEmail := range user.Emails.Nodes {
				privateEmailAddress := privateEmail.Email
				if !slices.Contains(emailsFound, privateEmailAddress) {
					emailsFound = append(emailsFound, privateEmailAddress)
				}
			}
		}
	}
	return emailsFound, err
}

// Run the specified query string against the GitLab server's GraphQL API. Returns the API's response as
// a raw (JSON) byte slice, so that the calling function can decode it to its expected type.
func (server GraphQlServer) RunGraphQlQuery(query string) (statusCode int, responseBody []byte, err error) {
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
		return statusCode, responseBody, fmt.Errorf("error trying to encode GraphQL query '%v' as JSON: '%w'", query, err)
	}
	// Setup the request
	req, err := http.NewRequest("POST", server.GraphQlUrl, bytes.NewBuffer(postJson))
	// req, err := http.NewRequest("POST", server.GraphQlUrl, strings.NewReader(string(postJson)))  // this also works
	if err != nil {
		wrappedErr := fmt.Errorf("error trying to create HTTP request to server '%v' with payload '%v': '%w'", server.GraphQlUrl, query, err)
		return statusCode, responseBody, wrappedErr
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", "Bearer "+server.GitlabToken)
	// Make the request
	slog.Debug("Making HTTP request:", slog.Any("httpRequest", req))
	res, err := client.Do(req)
	if err != nil {
		wrappedErr := fmt.Errorf("error making HTTP request to server '%v' with payload '%v': '%w'", server.GraphQlUrl, query, err)
		return statusCode, responseBody, wrappedErr
	}
	// Return the results
	statusCode = res.StatusCode
	defer res.Body.Close()
	responseBody, err = io.ReadAll(res.Body)
	if err != nil {
		wrappedErr := fmt.Errorf("error reading response from server '%v' with GraphQL query '%v': '%w'", server.GraphQlUrl, query, err)
		return statusCode, responseBody, wrappedErr
	}
	if res.StatusCode != http.StatusOK {
		err = fmt.Errorf("graphQL request to server '%v' with query '%v' returned status %d", server.GraphQlUrl, query, res.StatusCode)
		return statusCode, responseBody, err
	}
	err = getGraphQlErrors(responseBody)
	if err != nil {
		wrappedErr := fmt.Errorf("graphQL query '%v' received status code %d and errors: %w", query, res.StatusCode, err)
		return statusCode, responseBody, wrappedErr
	}
	slog.Debug("Successful HTTP response:", slog.Any(fmt.Sprint(res.StatusCode), responseBody))
	return statusCode, responseBody, err
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

// Replace consecutive occurences of whitespace characters with a single space
func consolidateWhitespace(s string) string {
	// strings.Fields() splits on any amount of white space
	return strings.Join(strings.Fields(strings.TrimSpace(s)), " ")
}
