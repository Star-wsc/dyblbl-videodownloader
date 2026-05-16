package models

type Platform string

const (
	PlatformDouyin  Platform = "douyin"
	PlatformBilibili Platform = "bilibili"
)

type VideoInfo struct {
	Title              string
	Author             string
	AuthorID           string
	CoverURL           string
	VideoURL           string
	AudioURL           string
	SelectedQuality    string
	AvailableQualities []string
	Platform           Platform
}

type DownloadResult struct {
	FilePath string
	FileSize int64
	Title    string
	Author   string
	Platform Platform
}
