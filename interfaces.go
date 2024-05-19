package main

type groupChecker interface {
	GetDirectGroupMembers(projectFullPath string) (groups []string, err error)
}

type userChecker interface {
	GetDirectUserMembers(projectFullPath string, userSource string) (usernamesFound []string, emailsFound []string, err error)
}
