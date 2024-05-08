package analysis

type CodeownersFileAnatomy struct {
	CodeownersFilePath   string
	CodeownersFileLines  []string
	FilePatterns         []string
	UserAndGroupPatterns []string
	EmailPatterns        []string
}
