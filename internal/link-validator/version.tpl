package link_validator

type VersionInfo struct {
	Version   string `json:"Version"`
	GitCommit string `json:"GitCommit"`
	BuildDate string `json:"BuildDate"`
}

var Version = VersionInfo{
	Version:   "0.0.0",
	GitCommit: "xxx_git_commit_xxx",
	BuildDate: "xxx_build_date_xxx",
}
