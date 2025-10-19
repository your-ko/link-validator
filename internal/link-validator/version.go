package link_validator

type VersionInfo struct {
	Version   string `json:"Version"`
	GitCommit string `json:"GitCommit"`
	BuildDate string `json:"BuildDate"`
}

var Version = VersionInfo{
	Version:   "0.14.0",
	GitCommit: "d23bdc2edd97d3f567393907aa10491fde6197ca",
	BuildDate: "2025-10-19T10:22:32Z",
}
