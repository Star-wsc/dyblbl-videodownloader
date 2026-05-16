package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Star-wsc/dyblbl-videodownloader/internal/config"
	"github.com/Star-wsc/dyblbl-videodownloader/internal/downloader"

	"github.com/spf13/cobra"
)

var douyinCmd = &cobra.Command{
	Use:   "douyin [url]",
	Short: "Download video from Douyin",
	Long:  `Download a video from Douyin (TikTok China) by providing the video URL.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runDouyinDownload,
}

var (
	douyinOutput   string
	douyinQuality  string
)

func init() {
	douyinCmd.Flags().StringVarP(&douyinOutput, "output", "o", "", "Output file path")
	douyinCmd.Flags().StringVarP(&douyinQuality, "quality", "q", "high", "Video quality (normal, high, super)")
	rootCmd.AddCommand(douyinCmd)
}

func runDouyinDownload(cmd *cobra.Command, args []string) error {
	url := args[0]
	cfg := config.GetConfig()

	d := downloader.NewDouyinDownloader()

	extractedURL := d.ExtractURLFromText(url)
	if extractedURL == "" {
		extractedURL = url
	}

	fmt.Printf("Parsing video: %s\n", extractedURL)

	ctx := context.Background()
	videoInfo, err := d.GetVideoData(ctx, extractedURL)
	if err != nil {
		return fmt.Errorf("failed to get video data: %w", err)
	}

	fmt.Printf("Title: %s\n", videoInfo.Title)
	fmt.Printf("Author: %s\n", videoInfo.Author)
	fmt.Printf("Quality: %s\n", videoInfo.SelectedQuality)

	if videoInfo.VideoURL == "" {
		return fmt.Errorf("no video URL found")
	}

	outputPath := douyinOutput
	if outputPath == "" {
		outputPath = generateOutputPath(cfg.DownloadDir, videoInfo.Title, videoInfo.Author, "douyin", "mp4")
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	fmt.Printf("Downloading to: %s\n", outputPath)

	cookies := make(map[string]string)
	if cfg.DouyinCookie != "" {
		for _, part := range strings.Split(cfg.DouyinCookie, ";") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			kv := strings.SplitN(part, "=", 2)
			if len(kv) == 2 {
				cookies[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
			}
		}
	}

	data, err := d.DownloadVideo(ctx, videoInfo.VideoURL, cookies)
	if err != nil {
		return fmt.Errorf("failed to download video: %w", err)
	}

	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		return fmt.Errorf("failed to save video: %w", err)
	}

	fmt.Printf("Download completed: %s\n", outputPath)
	return nil
}

func generateOutputPath(dir, title, author, platform, ext string) string {
	safeTitle := sanitizeFilename(title)
	safeAuthor := sanitizeFilename(author)

	if safeTitle == "" {
		safeTitle = "video"
	}
	if safeAuthor == "" {
		safeAuthor = "unknown"
	}

	filename := fmt.Sprintf("%s_%s_%s.%s", platform, safeAuthor, safeTitle, ext)
	return filepath.Join(dir, filename)
}

func sanitizeFilename(name string) string {
	invalid := []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|"}
	result := name
	for _, char := range invalid {
		result = strings.ReplaceAll(result, char, "_")
	}
	if len(result) > 100 {
		result = result[:100]
	}
	return strings.TrimSpace(result)
}
