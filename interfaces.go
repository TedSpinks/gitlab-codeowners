package main

type groupUserChecker interface {
	CheckForUsers(usernameList []string) (usernamesFound []string, err error)
	CheckForGroups(groupNameList []string) (groupsFound []string, err error)
}

type emailChecker interface {
	CheckForUsersByEmail(emailList []string) (emailsFound []string, err error)
}

type groupChecker interface {
	GetDirectGroupMembers(projectFullPath string) (groups []string, err error)
}

type userChecker interface {
	GetDirectUserMembers(projectFullPath string, userSource string) (usernamesFound []string, emailsFound []string, err error)
}
