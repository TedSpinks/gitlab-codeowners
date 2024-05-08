// This package contains methods to analyze a CODEOWNERS file. Assumes that the current directory is the
// root of a Git repo, which contains the CODEOWNERS file in one of GitLab's 3 supported locations - see
// https://docs.gitlab.com/ee/user/project/codeowners/#codeowners-file
package analysis

import (
	"fmt"
	"log/slog"
	"os"
)

var Co CodeownersFileAnatomy

func init() {
	err := Co.determineCodeownersPath()
	if err != nil {
		panic(err.Error())
	}
}

// Get the path to the local CODEOWNERS file.
func (co *CodeownersFileAnatomy) CoPath() (codeownersFilePath string) {
	return co.CodeownersFilePath
}

func (co *CodeownersFileAnatomy) determineCodeownersPath() error {
	supportedLocations := [...]string{"CODEOWNERS", "docs/CODEOWNERS", ".gitlab/CODEOWNERS"}
	for _, location := range supportedLocations {
		coExists, err := fileExists(location)
		if err != nil {
			slog.Debug(err.Error())
		}
		if coExists {
			co.CodeownersFilePath = location
			return nil
		}
	}
	return fmt.Errorf("unable to find a CODEOWNERS file at GitLab's 3 supported paths: %v", supportedLocations)
}

// Return whether or not the specified file can be found within the file system. Note that Linux has
// a case sensitive file system, but Mac (surprisingly) and Windows do not. To test this, try creating
// 2 files with the same spelling, but different cases. A case sensitive file system will allow this.
func fileExists(filePath string) (bool, error) {
	stat, err := os.Stat(filePath)
	if err == nil {
		if !stat.IsDir() {
			return true, nil
		} else {
			return false, fmt.Errorf("'%v' is a directory, not a file", filePath)
		}
	} else {
		return false, err
	}
}
