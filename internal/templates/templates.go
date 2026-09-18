package templates

const (
	RepoTag    = "RepoTag"
	RepoUrl    = "RepoUrl"
	RepoCommit = "RepoCommit"

	FailedTaskName   = "FailedTaskName"
	FailedQuorumName = "FailedQuorumName"

	StartedTaskName = "StartedTaskName"
)

type RepoTemplateVarsData struct {
	RepoTag    string
	RepoUrl    string
	RepoCommit string
}

// GetRepoTemplateVars returns every known variable, so that a template using a
// variable that is only filled in later (a hook referencing FailedTaskName from
// a shared env value, for example) renders empty instead of failing. An unknown
// variable is still an error.
func GetRepoTemplateVars(data RepoTemplateVarsData) map[string]string {
	vars := map[string]string{
		FailedTaskName:   "",
		FailedQuorumName: "",
		StartedTaskName:  "",
	}
	vars[RepoTag] = data.RepoTag
	vars[RepoUrl] = data.RepoUrl
	vars[RepoCommit] = data.RepoCommit
	return vars
}
