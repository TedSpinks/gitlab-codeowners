package main

type syntaxChecker interface {
	CheckCodeownersSyntax(codeownersPath string, projectPath string, branch string) (err error)
}

type groupChecker interface {
	GetDirectGroupMembers(projectFullPath string) (groups []string, err error)
}

type userChecker interface {
	GetDirectUserMembers(projectFullPath string, userSource string) (usernamesFound []string, emailsFound []string, err error)
}
