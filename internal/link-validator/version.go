package link_validator

type VersionInfo struct {
	Version   string `json:"Version"`
	GitCommit string `json:"GitCommit"`
	BuildDate string `json:"BuildDate"`
}

var Version = VersionInfo{
	Version:   "0.4.0",
	GitCommit: "705b95a913ca3395613f81fbef68187149879491",
	BuildDate: "2025-09-19T19:14:07Z",
}
