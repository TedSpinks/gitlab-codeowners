package rest

type Server struct {
	RestUrl     string // HTTPS URL for your GitLab instance's REST API.
	GitlabToken string // GitLab token for connecting to the REST API (scope=read_api, role=Developer)
	Timeout     int    // Timeout for REST requests, in seconds
}

// JSON documentation:
//https://docs.gitlab.com/ee/api/projects.html#get-single-project

type Projects []Project

type Project struct {
	Id                int     `json:"id"`
	PathWithNamespace string  `json:"string path_with_namespace"`
	SharedWithGroups  []Group `json:"shared_with_groups"`
}

type Group struct {
	GroupId          int    `json:"group_id"`
	GroupName        string `json:"group_name"`
	GroupFullPath    string `json:"group_full_path"`
	GroupAccessLevel int    `json:"group_access_level"`
}
