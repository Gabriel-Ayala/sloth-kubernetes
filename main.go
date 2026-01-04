package main

import "github.com/chalkan3/sloth-kubernetes/cmd"

// Version information - set by goreleaser ldflags
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
	builtBy = "unknown"
)

func main() {
	cmd.SetVersionInfo(version, commit, date, builtBy)
	cmd.Execute()
}
