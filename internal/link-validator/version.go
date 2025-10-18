package link_validator

type VersionInfo struct {
	Version   string `json:"Version"`
	GitCommit string `json:"GitCommit"`
	BuildDate string `json:"BuildDate"`
}

var Version = VersionInfo{
	Version:   "0.12.0",
	GitCommit: "e7586e9c9a50017653374090bc451d423227390e",
	BuildDate: "2025-10-18T05:44:32Z",
}
