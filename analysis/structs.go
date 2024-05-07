package analysis

type Anatomy struct {
	CodeownersFilePath   string
	CodeownersFileLines  []string
	FilePatterns         []string
	UserAndGroupPatterns []string
	EmailPatterns        []string
}
