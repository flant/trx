package templates

const (
	RepoTag    = "RepoTag"
	RepoUrl    = "RepoUrl"
	RepoCommit = "RepoCommit"

	FailedTaskName   = "FailedTaskName"
	FailedQuorumName = "FailedQuorumName"
)

type RepoTemplateVarsData struct {
	RepoTag    string
	RepoUrl    string
	RepoCommit string
}

func GetRepoTemplateVars(data RepoTemplateVarsData) map[string]string {
	vars := make(map[string]string)
	vars[RepoTag] = data.RepoTag
	vars[RepoUrl] = data.RepoUrl
	vars[RepoCommit] = data.RepoCommit
	return vars
}
