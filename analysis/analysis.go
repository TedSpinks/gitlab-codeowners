// This package contains methods to analyze a CODEOWNERS file. Assumes that the current directory is the
// root of a Git repo, which contains the CODEOWNERS file in one of GitLab's 3 supported locations - see
// https://docs.gitlab.com/ee/user/project/codeowners/#codeowners-file
package analysis

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
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

func (co *CodeownersFileAnatomy) Analyze() {
	// Read in the CODEOWNERS file
	if len(co.CodeownersFileLines) == 0 {
		co.readCodeownersFile()
	}
	// Define sets (map of bool) to record unique patterns with no dupes
	sectionHeadingsMap := map[string]bool{}
	filePatternsMap := map[string]bool{}
	userAndGroupPatternsMap := map[string]bool{}
	emailPatternsMap := map[string]bool{}
	ignoredPatternsMap := map[string]bool{}
	// Analyze each line of the CODEOWNERS file
	for _, l := range co.CodeownersFileLines {
		sectionHeading, filePattern, ownerPatterns := splitCodeownersLine(l)
		sectionHeadingsMap[sectionHeading] = true
		filePatternsMap[filePattern] = true
		usersOrGroups, emails, ignored := splitOwnerPatterns(ownerPatterns)
		for _, ug := range usersOrGroups {
			userAndGroupPatternsMap[ug] = true
		}
		for _, e := range emails {
			emailPatternsMap[e] = true
		}
		for _, i := range ignored {
			ignoredPatternsMap[i] = true
		}
	}
	// Write unique patterns to co object
	co.Analyzed = true
	co.SectionHeadings = boolMapToSlice(sectionHeadingsMap)
	co.FilePatterns = boolMapToSlice(filePatternsMap)
	co.UserAndGroupPatterns = boolMapToSlice(userAndGroupPatternsMap)
	co.EmailPatterns = boolMapToSlice(emailPatternsMap)
	co.IgnoredPatterns = boolMapToSlice(ignoredPatternsMap)
}

func boolMapToSlice(m map[string]bool) []string {
	i := 0
	keys := make([]string, len(m))
	for k, _ := range m {
		keys[i] = k
		i++
	}
	return keys
}

func (co *CodeownersFileAnatomy) readCodeownersFile() {
	content, err := os.ReadFile(co.CodeownersFilePath)
	if err != nil {
		err = fmt.Errorf("unable to read CODEOWNERS file at path '%v': %w", co.CodeownersFilePath, err)
		panic(err.Error())
	}
	// Cast the []byte content to a string, and split it on Windows + Linux line endings
	co.CodeownersFileLines = strings.Split(strings.ReplaceAll(string(content), "\r\n", "\n"), "\n")
}

// Split the owner portion of a CODEOWNERS line into its individual @user/@group and email patterns
// Note: Owner patterns that don't contain '@' are ignored by GitLab. This behavior is described
// here: https://docs.gitlab.com/ee/user/project/codeowners/reference.html#example-codeowners-file
func splitOwnerPatterns(ownerPatterns string) (usersOrGroups []string, emails []string, ignored []string) {
	for _, o := range strings.Fields(ownerPatterns) {
		if strings.HasPrefix(o, "@") {
			usersOrGroups = append(usersOrGroups, o)
		} else if strings.Contains(o, "@") {
			emails = append(emails, o)
		} else {
			ignored = append(ignored, o)
		}
	}
	return
}

// Split each CODEOWNERS line into its main parts, with a [section heading] or file pattern on the left,
// and owner patterns on the right.
func splitCodeownersLine(line string) (sectionHeading string, filePattern string, ownerPatterns string) {
	line = strings.TrimSpace(line)
	// Skip any blank/whitespace or comment lines
	if line == "" || strings.HasPrefix(line, "#") {
		return
	}
	splitPosition := 0
	prevCharIsEscape := false
	firstCharIsHat := false // hat aka carat
	sectionHeadingStarted := false
	sectionHeadingEnded := false
	// Find the split position within the line
	for i, c := range line {
		if i > 0 && line[i-1] == '\\' {
			prevCharIsEscape = true
		}
		if i == 0 && c == '^' {
			firstCharIsHat = true
		}
		// A section heading is indicated by a line starting with "[" or "^["
		if (i == 0 && c == '[') || (i == 1 && firstCharIsHat && c == '[') {
			sectionHeadingStarted = true
		}
		if sectionHeadingStarted && !prevCharIsEscape && c == ']' {
			sectionHeadingEnded = true
		}
		// if position is not currently within a [section heading]...
		if !(sectionHeadingStarted && !sectionHeadingEnded) {
			// ...and it is an un-escaped space or tab, then this is the split position!
			if !prevCharIsEscape && (c == ' ' || c == '\t') {
				splitPosition = i
				break
			}
		}
	}

	// If no split position was found, the whole line is either a [section heading] or a naked file pattern
	if splitPosition == 0 {
		if sectionHeadingStarted {
			sectionHeading = line
		} else {
			filePattern = line
		}
		return
	}

	// Split the line and return results
	if sectionHeadingStarted {
		sectionHeading = line[:splitPosition]
	} else {
		filePattern = line[:splitPosition]
	}
	ownerPatterns = line[splitPosition+1:]
	return
}
