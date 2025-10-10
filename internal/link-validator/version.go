package link_validator

type VersionInfo struct {
	Version   string `json:"Version"`
	GitCommit string `json:"GitCommit"`
	BuildDate string `json:"BuildDate"`
}

var Version = VersionInfo{
	Version:   "0.7.0",
	GitCommit: "da78df46bd59a59f9ebf076a0b96f9af7b6d915f",
	BuildDate: "2025-10-04T18:20:17Z",
}
