package link_validator

type VersionInfo struct {
	Version   string `json:"Version"`
	GitCommit string `json:"GitCommit"`
	BuildDate string `json:"BuildDate"`
}

var Version = VersionInfo{
	Version:   "0.10.0",
	GitCommit: "13d3032e45eaf4c6c45e7e769f271936f6914b73",
	BuildDate: "2025-10-14T16:26:30Z",
}
