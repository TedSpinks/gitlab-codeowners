package rest

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	neturl "net/url"

	// "slices"
	"strings"
	"time"
)

// Return the full path (ex: top-group/sub-group/etc-group) of all the groups that are direct members of the
// specified project.
func (server Server) GetDirectGroupMembers(projectFullPath string) (groups []string, err error) {
	project, err := server.GetProjectByPath(projectFullPath)
	if err != nil {
		err = fmt.Errorf("GetDirectGroupMembers(): %w", err)
		return
	}
	if project == nil {
		return
	}
	fmt.Println(project)
	fmt.Println(project.SharedWithGroups)
	for _, group := range project.SharedWithGroups {
		groups = append(groups, group.GroupFullPath)
	}
	return
}

// Look up a project by its full path (ex: my-group/my-subgroup/my-project). If there is no project with the
// specified path that is visible to the server.GitlabToken identity, then the "project" return will be nil.
// Note: in order for project to be allowed to be nil, I had to make it a pointer.
func (server Server) GetProjectByPath(projectFullPath string) (project *Project, err error) {
	projectFullPath = strings.TrimPrefix(projectFullPath, "/")
	// A valid group/project path must have at least one slash
	if !strings.Contains(projectFullPath, "/") {
		panic("GetProjectByPath() requires a path in the format of group/project or group/subgroup/project, invalid path: '" + projectFullPath + "'")
	}
	// URL-encode the slashes in the group path
	endpointPath := "/projects/" + strings.Replace(projectFullPath, "/", "%2F", -1)
	// Make the REST request
	_, jsonResponse, err := server.RestRequest(endpointPath, "GET", "")
	if err != nil {
		err = fmt.Errorf("GetProjectById() failed looking up project path '%v': %w", projectFullPath, err)
		return nil, err
	}
	err = json.Unmarshal(jsonResponse, &project)
	if err != nil {
		err = fmt.Errorf("GetProjectById() could not decode JSON response '%v' when looking up project path '%v': %w",
			string(jsonResponse), projectFullPath, err)
		return nil, err
	}
	return project, nil
}

// Look up a project by its ID. If there is no project with the specified ID that is visible to the
// server.GitlabToken identity, then the "project" return will be nil.
// Note: in order for project to be allowed to be nil, I had to make it a pointer.
func (server Server) GetProjectById(id int) (project *Project, err error) {
	path := fmt.Sprintf("/projects/%d", id)
	_, jsonResponse, err := server.RestRequest(path, "GET", "")
	if err != nil {
		err = fmt.Errorf("GetProjectById() failed looking up project ID '%d': %w", id, err)
		return nil, err
	}
	err = json.Unmarshal(jsonResponse, &project)
	if err != nil {
		err = fmt.Errorf("GetProjectById() could not decode JSON response '%v' when looking up project ID '%d': %w", string(jsonResponse), id, err)
		return nil, err
	}
	return project, nil
}

// Make the specified request against the GitLab server's REST API. Returns the API's response as
// a raw (JSON) byte slice, so that the calling function can decode it to its expected type.
func (server Server) RestRequest(path string, method string, jsonPayload string) (statusCode int, jsonResponse []byte, err error) {
	endpointUrl := strings.TrimSuffix(server.RestUrl, "/") + "/" + strings.TrimPrefix(path, "/")
	validateUrlWithPath(endpointUrl)
	validateRestMethod(method)
	// Setup the request
	client := &http.Client{
		Timeout: time.Second * time.Duration(server.Timeout),
	}
	req, err := http.NewRequest(method, endpointUrl, strings.NewReader(jsonPayload))
	if err != nil {
		wrappedErr := fmt.Errorf("error trying to create REST request to '%v' with payload '%v': '%w'", endpointUrl, jsonPayload, err)
		return statusCode, jsonResponse, wrappedErr
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", "Bearer "+server.GitlabToken)
	// Make the request
	slog.Debug("Making HTTP request:", slog.Any("httpRequest", req))
	res, err := client.Do(req)
	if err != nil {
		wrappedErr := fmt.Errorf("error making REST request to '%v' with payload '%v': '%w'", endpointUrl, jsonPayload, err)
		return statusCode, jsonResponse, wrappedErr
	}
	// Return the results
	statusCode = res.StatusCode
	defer res.Body.Close()
	jsonResponse, err = io.ReadAll(res.Body)
	if err != nil {
		wrappedErr := fmt.Errorf("error reading response from request '%v' with payload '%v': '%w'", endpointUrl, jsonPayload, err)
		return statusCode, jsonResponse, wrappedErr
	}
	slog.Debug("HTTP response received:", slog.Any(fmt.Sprint(res.StatusCode), jsonResponse))
	if res.StatusCode != http.StatusOK {
		err = fmt.Errorf("request '%v' with payload '%v' returned status %d and response '%v'", endpointUrl, jsonPayload, res.StatusCode, string(jsonResponse))
		return statusCode, jsonResponse, err
	}
	return statusCode, jsonResponse, err
}

// Exit the program if the provided URL is not valid
func validateUrlWithPath(url string) {
	u, err := neturl.Parse(url)
	if err != nil {
		log.Fatalf("cannot parse URL '%v':", url)
	}
	if u.Scheme == "" || u.Host == "" {
		log.Fatalf("invalid URL '%v':", url)
	}
	if u.Path == "" {
		log.Fatalf("URL does not contain a path: '%v'", url)
	}
}

// Exit the program if the provided REST method is not valid
func validateRestMethod(method string) {
	switch method {
	case "GET", "PUT", "POST", "DELETE", "PATCH":
		// valid
	default:
		// developer error
		panic("invalid REST method, should be one of GET, PUT, POST, DELETE, PATCH: '" + method + "'")
	}
}
