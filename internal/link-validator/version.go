package link_validator

type VersionInfo struct {
	Version   string `json:"Version"`
	GitCommit string `json:"GitCommit"`
	BuildDate string `json:"BuildDate"`
}

var Version = VersionInfo{
	Version:   "0.5.0",
	GitCommit: "627b29d7c1bc4cb414a0fab8c5d1051bc0aef43d",
	BuildDate: "2025-09-24T07:14:27Z",
}
