package engine

var helmSubjectOverrides = map[string]helmSubject{
	"abbrev":                     subjectLast,
	"abbrevboth":                 subjectLast,
	"chunk":                      subjectLast,
	"contains":                   subjectLast,
	"date":                       subjectLast,
	"dateInZone":                 subjectAt(1),
	"dateModify":                 subjectLast,
	"date_in_zone":               subjectAt(1),
	"date_modify":                subjectLast,
	"decryptAES":                 subjectLast,
	"default":                    subjectLast,
	"dig":                        subjectLast,
	"encryptAES":                 subjectLast,
	"has":                        subjectLast,
	"hasPrefix":                  subjectLast,
	"hasSuffix":                  subjectLast,
	"indent":                     subjectLast,
	"join":                       subjectLast,
	"kindIs":                     subjectLast,
	"mustChunk":                  subjectLast,
	"mustDateModify":             subjectLast,
	"mustRegexFind":              subjectAt(1),
	"mustRegexFindAll":           subjectAt(1),
	"mustRegexMatch":             subjectAt(1),
	"mustRegexReplaceAll":        subjectAt(1),
	"mustRegexReplaceAllLiteral": subjectAt(1),
	"mustRegexSplit":             subjectAt(1),
	"mustToDate":                 subjectLast,
	"must_date_modify":           subjectLast,
	"nindent":                    subjectLast,
	"plural":                     subjectLast,
	"pluck":                      subjectAt(1),
	"regexFind":                  subjectAt(1),
	"regexFindAll":               subjectAt(1),
	"regexMatch":                 subjectAt(1),
	"regexReplaceAll":            subjectAt(1),
	"regexReplaceAllLiteral":     subjectAt(1),
	"regexSplit":                 subjectAt(1),
	"repeat":                     subjectLast,
	"replace":                    subjectLast,
	"required":                   subjectLast,
	"semverCompare":              subjectLast,
	"split":                      subjectLast,
	"splitList":                  subjectLast,
	"splitn":                     subjectLast,
	"substr":                     subjectLast,
	"ternary":                    subjectLast,
	"toDate":                     subjectLast,
	"trimAll":                    subjectLast,
	"trimPrefix":                 subjectLast,
	"trimSuffix":                 subjectLast,
	"trimall":                    subjectLast,
	"trunc":                      subjectLast,
	"typeIs":                     subjectLast,
	"typeIsLike":                 subjectLast,
	"wrap":                       subjectLast,
	"wrapWith":                   subjectLast,

	"all":      subjectNone,
	"any":      subjectNone,
	"cat":      subjectNone,
	"coalesce": subjectNone,
	"concat":   subjectNone,
	"dict":     subjectNone,
	"list":     subjectNone,
	"tuple":    subjectNone,
}

var knownFuncNames = func() map[string]struct{} {
	r := NewFuncRegistry()
	names := make(map[string]struct{}, len(r.funcs)+len(bridged)+8)
	for name := range r.funcs {
		names[name] = struct{}{}
	}
	for name := range bridged {
		names[name] = struct{}{}
	}
	for _, name := range []string{"include", "tpl", "lookup", "Files.Get", "Files.Glob",
		"Files.AsConfig", "Files.AsSecrets", "Files.Lines", "Capabilities.APIVersions.Has"} {
		names[name] = struct{}{}
	}
	return names
}()

func KnownFunc(name string) bool {
	_, ok := knownFuncNames[name]
	return ok
}
