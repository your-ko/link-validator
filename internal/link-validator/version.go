package link_validator

type VersionInfo struct {
	Version   string `json:"Version"`
	GitCommit string `json:"GitCommit"`
	BuildDate string `json:"BuildDate"`
}

var Version = VersionInfo{
	Version:   "0.8.0",
	GitCommit: "0afd5a78837951783f4393e87b976d91b98f84f0",
	BuildDate: "2025-10-10T06:38:08Z",
}
