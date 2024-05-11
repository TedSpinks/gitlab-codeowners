package analysis

type CodeownersFileAnatomy struct {
	CodeownersFilePath   string
	Analyzed             bool
	CodeownersFileLines  []string
	SectionHeadings      []string
	FilePatterns         []string
	UserAndGroupPatterns []string
	EmailPatterns        []string
	IgnoredPatterns      []string
}
