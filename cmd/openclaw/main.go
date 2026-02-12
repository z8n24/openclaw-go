package main

import (
	"os"

	"github.com/user/openclaw-go/internal/cli"
)

// 版本信息 (可通过 ldflags 设置)
var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	// 设置版本信息
	cli.SetVersionInfo(version, commit, date)

	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
