package link_validator

type VersionInfo struct {
	Version   string `json:"Version"`
	GitCommit string `json:"GitCommit"`
	BuildDate string `json:"BuildDate"`
}

var Version = VersionInfo{
	Version:   "0.6.0",
	GitCommit: "0fa82e60170d41dd53a0d2b780d5eeeab2061125",
	BuildDate: "2025-09-27T14:16:13Z",
}
