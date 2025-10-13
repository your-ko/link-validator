package link_validator

type VersionInfo struct {
	Version   string `json:"Version"`
	GitCommit string `json:"GitCommit"`
	BuildDate string `json:"BuildDate"`
}

var Version = VersionInfo{
	Version:   "0.9.0",
	GitCommit: "c91abea10ddbc208c28647be03b15b2bac4b117b",
	BuildDate: "2025-10-13T06:56:00Z",
}
