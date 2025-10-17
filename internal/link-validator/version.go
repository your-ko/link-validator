package link_validator

type VersionInfo struct {
	Version   string `json:"Version"`
	GitCommit string `json:"GitCommit"`
	BuildDate string `json:"BuildDate"`
}

var Version = VersionInfo{
	Version:   "0.11.0",
	GitCommit: "bcf7894c459ecff73a92b859a7d0fddf8b9f65bd",
	BuildDate: "2025-10-17T18:38:14Z",
}
