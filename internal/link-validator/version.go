package link_validator

type VersionInfo struct {
	Version   string `json:"Version"`
	GitCommit string `json:"GitCommit"`
	BuildDate string `json:"BuildDate"`
}

var Version = VersionInfo{
	Version:   "0.3.0",
	GitCommit: "37516e0b7c3dcc31bf0bf975605ac293730df8ca",
	BuildDate: "2025-09-19T19:07:58Z",
}
