package models

type Platform string

const (
	PlatformDouyin  Platform = "douyin"
	PlatformBilibili Platform = "bilibili"
	PlatformTikTok  Platform = "tiktok"
)

type VideoInfo struct {
	Title              string
	Author             string
	AuthorID           string
	CoverURL           string
	VideoURL           string
	AudioURL           string
	DownloadURL        string
	SelectedQuality    string
	AvailableQualities []string
	Cookies            map[string]string
	Platform           Platform
}

type DownloadResult struct {
	FilePath string
	FileSize int64
	Title    string
	Author   string
	Platform Platform
}
