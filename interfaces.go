package main

type groupUserChecker interface {
	CheckForUsers(usernameList []string) (usernamesFound []string, err error)
	CheckForGroups(groupNameList []string) (groupsFound []string, err error)
}

type emailChecker interface {
	CheckForUsersByEmail(emailList []string) (emailsFound []string, err error)
}
