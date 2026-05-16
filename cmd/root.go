package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var version = "dev"

var rootCmd = &cobra.Command{
	Use:     "vdl",
	Short:   "Video Downloader CLI - Download videos from Douyin and Bilibili",
	Long:    `A command-line tool for downloading videos from Douyin (TikTok China) and Bilibili.`,
	Version: version,
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.CompletionOptions.HiddenDefaultCmd = true
}

func printError(msg string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "Error: "+msg+"\n", args...)
}

func printSuccess(msg string, args ...interface{}) {
	fmt.Printf(msg+"\n", args...)
}
