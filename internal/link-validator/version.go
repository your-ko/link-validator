package link_validator

type VersionInfo struct {
	Version   string `json:"Version"`
	GitCommit string `json:"GitCommit"`
	BuildDate string `json:"BuildDate"`
}

var Version = VersionInfo{
	Version:   "0.2.0",
	GitCommit: "05dda04dfac323239a226006b1ac0648d8d4cdb1",
	BuildDate: "2025-09-19T19:05:00Z",
}
