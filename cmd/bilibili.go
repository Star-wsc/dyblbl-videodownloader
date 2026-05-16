package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Star-wsc/dyblbl-videodownloader/internal/config"
	"github.com/Star-wsc/dyblbl-videodownloader/internal/downloader"

	"github.com/spf13/cobra"
)

var bilibiliCmd = &cobra.Command{
	Use:   "bilibili [url]",
	Short: "Download video from Bilibili",
	Long:  `Download a video from Bilibili by providing the video URL.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runBilibiliDownload,
}

var (
	bilibiliOutput   string
	bilibiliQuality  string
)

func init() {
	bilibiliCmd.Flags().StringVarP(&bilibiliOutput, "output", "o", "", "Output file path")
	bilibiliCmd.Flags().StringVarP(&bilibiliQuality, "quality", "q", "1080p", "Video quality (480p, 720p, 1080p, 4k)")
	rootCmd.AddCommand(bilibiliCmd)
}

func runBilibiliDownload(cmd *cobra.Command, args []string) error {
	url := args[0]
	cfg := config.GetConfig()

	d := downloader.NewBilibiliDownloader()

	if cfg.BilibiliCookie != "" {
		d.SetCookies(cfg.BilibiliCookie)
	}

	fmt.Printf("Parsing video: %s\n", url)

	ctx := context.Background()
	videoInfo, err := d.GetVideoData(ctx, url)
	if err != nil {
		return fmt.Errorf("failed to get video data: %w", err)
	}

	fmt.Printf("Title: %s\n", videoInfo.Title)
	fmt.Printf("Author: %s\n", videoInfo.Author)
	fmt.Printf("Quality: %s\n", videoInfo.SelectedQuality)

	if videoInfo.VideoURL == "" {
		return fmt.Errorf("no video URL found")
	}

	outputPath := bilibiliOutput
	if outputPath == "" {
		outputPath = generateOutputPath(cfg.DownloadDir, videoInfo.Title, videoInfo.Author, "bilibili", "mp4")
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	fmt.Printf("Downloading to: %s\n", outputPath)

	// 使用DownloadWithMerge下载视频和音频并合并
	if err := d.DownloadWithMerge(ctx, videoInfo.VideoURL, videoInfo.AudioURL, outputPath); err != nil {
		return fmt.Errorf("failed to download video: %w", err)
	}

	fmt.Printf("Download completed: %s\n", outputPath)
	return nil
}
