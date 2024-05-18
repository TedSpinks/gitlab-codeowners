package graphql

type Server struct {
	GraphQlUrl  string // HTTPS URL for your GitLab instance's GraphQL API.
	GitlabToken string // GitLab token for connecting to the GraphQL API (scope=read_api, role=Developer)
	Timeout     int    // Timeout for GraphQL requests, in seconds
}

type ValidateCodeownersResponse struct {
	Data struct {
		Project struct {
			Repository struct {
				// This is a pointer so that we can check for a nil value (indicates a bad CODEOWNERS file path)
				ValidateCodeownerFile *ValidateCodeownersFile `json:"validateCodeownerFile"`
			} `json:"repository"`
		} `json:"project"`
	} `json:"data"`
}

type ValidateCodeownersFile struct {
	Total            int `json:"total"`
	ValidationErrors []struct {
		Code  string `json:"code"`
		Lines []int  `json:"lines"`
	} `json:"validationErrors"`
}

type GroupQueryResponse struct {
	Data struct {
		Group struct {
			Id         string `json:"id"`
			Name       string `json:"name"`
			Path       string `json:"path"`
			FullName   string `json:"fullName"`
			FullPath   string `json:"fullPath"`
			Visibility string `json:"visibility"`
		} `json:"group"`
	} `json:"data"`
}

type UserQueryResponse struct {
	Data struct {
		Users struct {
			PageInfo struct {
				EndCursor   string `json:"endCursor"`
				StartCursor string `json:"startCursor"`
				HasNextPage bool   `json:"hasNextPage"`
			} `json:"pageInfo"`
			Nodes []struct {
				Id          string `json:"id"`
				Username    string `json:"username"`
				PublicEmail string `json:"publicEmail"`
				Emails      struct {
					Nodes []struct {
						Email string `json:"email"`
					} `json:"nodes"`
				} `json:"emails"`
			} `json:"nodes"`
		} `json:"users"`
	} `json:"data"`
}

type qraphqlQuery struct {
	Query string `json:"query"`
}

type QueryErrors struct {
	// Example error response (HTTP status 200, despite errors)
	// {"errors":[{"message":"Expected NAME, actual: LBRACKET (\"[\") at [1, 135]","locations":[{"line":1,"column":135}]}]}
	Errors []struct {
		Message   string `json:"message"`
		Locations []struct {
			Line   int `json:"line"`
			Column int `json:"column"`
		} `json:"locations"`
	} `json:"errors"`
}
