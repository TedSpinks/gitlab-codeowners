package gitlab

type GraphQlServer struct {
	GraphQlUrl  string // HTTPS URL for your GitLab instance's GraphQL API.
	GitlabToken string // GitLab token for connecting to the GraphQL API (scope=read_api, role=Developer)
	Timeout     int    // Timeout for GraphQL requests, in seconds
}

type UserQueryResponse struct {
	Data struct {
		Users struct {
			PageInfo PageInfo   `json:"pageInfo"`
			Nodes    []UserNode `json:"nodes"`
		} `json:"users"`
	} `json:"data"`
}

type PageInfo struct {
	EndCursor   string `json:"endCursor"`
	StartCursor string `json:"startCursor"`
	HasNextPage bool   `json:"hasNextPage"`
}

type UserNode struct {
	Id          string `json:"id"`
	Username    string `json:"username"`
	PublicEmail string `json:"publicEmail"`
	Emails      struct {
		Nodes []EmailNode `json:"nodes"`
	} `json:"emails"`
}

type EmailNode struct {
	Email string `json:"email"`
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
