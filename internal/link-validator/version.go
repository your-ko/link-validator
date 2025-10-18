package link_validator

type VersionInfo struct {
	Version   string `json:"Version"`
	GitCommit string `json:"GitCommit"`
	BuildDate string `json:"BuildDate"`
}

var Version = VersionInfo{
	Version:   "0.13.0",
	GitCommit: "cf46b6f3ae36ac63f73872e49ff54e3065536831",
	BuildDate: "2025-10-18T08:41:03Z",
}
