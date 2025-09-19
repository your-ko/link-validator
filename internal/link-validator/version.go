package link_validator

type VersionInfo struct {
	Version   string `json:"Version"`
	GitCommit string `json:"GitCommit"`
	BuildDate string `json:"BuildDate"`
}

var Version = VersionInfo{
	Version:   "0.1.0",
	GitCommit: "d10116ef47b4d94f5d2a83174e034b18c32856d8",
	BuildDate: "2025-09-19T19:01:53Z",
}
