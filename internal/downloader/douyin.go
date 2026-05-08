package downloader

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"video-downloader-cli/internal/models"
)

type DouyinDownloader struct {
	client *http.Client
}

func NewDouyinDownloader() *DouyinDownloader {
	return &DouyinDownloader{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

var mobileHeaders = map[string]string{
	"User-Agent":      "Mozilla/5.0 (Linux; Android 10; SM-G981B) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Mobile Safari/537.36",
	"Accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8",
	"Accept-Language": "zh-CN,zh;q=0.9",
}

var videoHeaders = map[string]string{
	"User-Agent":      "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	"Referer":         "https://www.douyin.com/",
	"Accept":          "*/*",
	"Accept-Encoding": "identity",
	"Connection":      "keep-alive",
}

var apiList = []string{
	"https://api.douyin.wtf/api?url=%s",
	"https://www.douyin.wtf/api?url=%s",
}

func (d *DouyinDownloader) ExtractURLFromText(text string) string {
	pattern := regexp.MustCompile(`https?://v\.douyin\.com/[A-Za-z0-9_-]+/?`)
	match := pattern.FindString(text)
	if match != "" {
		return strings.TrimRight(match, "/")
	}

	pattern2 := regexp.MustCompile(`https?://www\.douyin\.com/video/(\d+)`)
	match2 := pattern2.FindString(text)
	if match2 != "" {
		return match2
	}

	return ""
}

func (d *DouyinDownloader) GetVideoData(ctx context.Context, videoURL string) (*models.VideoInfo, error) {
	result, err := d.ParseViaAPI(ctx, videoURL)
	if err == nil && result.VideoURL != "" {
		return result, nil
	}

	return nil, fmt.Errorf("failed to parse video data")
}

func (d *DouyinDownloader) ParseViaAPI(ctx context.Context, videoURL string) (*models.VideoInfo, error) {
	escapedURL := url.QueryEscape(videoURL)

	for _, apiTemplate := range apiList {
		apiURL := fmt.Sprintf(apiTemplate, escapedURL)

		req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
		if err != nil {
			continue
		}
		req.Header.Set("User-Agent", "Mozilla/5.0")

		resp, err := d.client.Do(req)
		if err != nil {
			continue
		}

		if resp.StatusCode == 200 {
			body, err := io.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil {
				continue
			}

			var data map[string]interface{}
			if err := json.Unmarshal(body, &data); err != nil {
				continue
			}

			videoInfo := d.extractAPIData(data)
			if videoInfo != nil && videoInfo.VideoURL != "" {
				return videoInfo, nil
			}
		}
		resp.Body.Close()
	}

	return nil, fmt.Errorf("all APIs failed")
}

func (d *DouyinDownloader) extractAPIData(data map[string]interface{}) *models.VideoInfo {
	videoData, ok := data["data"].(map[string]interface{})
	if !ok {
		videoData = data
	}

	if dataList, ok := data["data"].([]interface{}); ok && len(dataList) > 0 {
		if vd, ok := dataList[0].(map[string]interface{}); ok {
			videoData = vd
		}
	}

	if videoData == nil {
		return nil
	}

	videoInfo, _ := videoData["video"].(map[string]interface{})
	if videoInfo == nil {
		return nil
	}

	videoURLs := d.extractVideoURLs(videoInfo)

	author, _ := videoData["author"].(map[string]interface{})
	authorNickname := ""
	if author != nil {
		authorNickname, _ = author["nickname"].(string)
	}

	title, _ := videoData["desc"].(string)
	if title == "" {
		title, _ = videoData["title"].(string)
	}

	return &models.VideoInfo{
		Title:              title,
		Author:             authorNickname,
		VideoURL:           videoURLs["high"],
		AvailableQualities: getKeys(videoURLs),
		Platform:           models.PlatformDouyin,
	}
}

func (d *DouyinDownloader) extractVideoURLs(videoInfo map[string]interface{}) map[string]string {
	urls := make(map[string]string)

	playAddr, _ := videoInfo["play_addr"].(map[string]interface{})
	if playAddr == nil {
		playAddr, _ = videoInfo["playAddr"].(map[string]interface{})
	}

	if playAddr != nil {
		urlList, _ := playAddr["url_list"].([]interface{})
		if len(urlList) > 0 {
			videoURL, _ := urlList[0].(string)
			videoURL = d.processVideoURL(videoURL)
			urls["normal"] = videoURL
			urls["high"] = videoURL
			urls["super"] = videoURL
		}
	}

	return urls
}

func (d *DouyinDownloader) processVideoURL(videoURL string) string {
	if strings.HasPrefix(videoURL, "//") {
		videoURL = "https:" + videoURL
	}
	videoURL = strings.Replace(videoURL, "playwm", "play", -1)
	return videoURL
}

func getKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func (d *DouyinDownloader) DownloadVideo(ctx context.Context, videoURL string, cookies map[string]string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", videoURL, nil)
	if err != nil {
		return nil, err
	}

	for k, v := range videoHeaders {
		req.Header.Set(k, v)
	}

	for name, value := range cookies {
		req.AddCookie(&http.Cookie{Name: name, Value: value})
	}

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 && resp.StatusCode != 206 {
		return nil, fmt.Errorf("HTTP status: %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}
