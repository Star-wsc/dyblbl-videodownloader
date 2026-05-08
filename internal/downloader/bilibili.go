package downloader

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"time"

	"video-downloader-cli/internal/models"
)

type BilibiliDownloader struct {
	client  *http.Client
	cookies string
}

func NewBilibiliDownloader() *BilibiliDownloader {
	return &BilibiliDownloader{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (d *BilibiliDownloader) SetCookies(cookies string) {
	d.cookies = cookies
}

func (d *BilibiliDownloader) GetVideoData(ctx context.Context, videoURL string) (*models.VideoInfo, error) {
	bvid, err := d.extractBVID(videoURL)
	if err != nil {
		return nil, err
	}

	aid, cid, title, author, coverURL, err := d.getVideoInfo(ctx, bvid)
	if err != nil {
		return nil, err
	}

	videoURLs, audioURL, qualities, selectedQuality, err := d.getVideoURLs(ctx, aid, cid, bvid, "1080p")
	if err != nil {
		return nil, err
	}

	return &models.VideoInfo{
		Title:              title,
		Author:             author,
		CoverURL:           coverURL,
		VideoURL:           videoURLs,
		AudioURL:           audioURL,
		SelectedQuality:    selectedQuality,
		AvailableQualities: qualities,
		Platform:           models.PlatformBilibili,
	}, nil
}

func (d *BilibiliDownloader) extractBVID(urlStr string) (string, error) {
	patterns := []string{
		`bilibili\.com/video/(BV[a-zA-Z0-9]+)`,
		`b23\.tv/(BV[a-zA-Z0-9]+)`,
		`(BV[a-zA-Z0-9]+)`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(urlStr)
		if len(matches) > 1 {
			return matches[1], nil
		}
	}

	return "", fmt.Errorf("cannot extract BVID from URL: %s", urlStr)
}

func (d *BilibiliDownloader) getVideoInfo(ctx context.Context, bvid string) (aid, cid int, title, author, coverURL string, err error) {
	apiURL := fmt.Sprintf("https://api.bilibili.com/x/web-interface/view?bvid=%s", bvid)

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return 0, 0, "", "", "", err
	}

	d.setCommonHeaders(req)

	resp, err := d.client.Do(req)
	if err != nil {
		return 0, 0, "", "", "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, 0, "", "", "", err
	}

	var result struct {
		Code int `json:"code"`
		Data struct {
			Aid   int    `json:"aid"`
			Cid   int    `json:"cid"`
			Title string `json:"title"`
			Pic   string `json:"pic"`
			Owner struct {
				Name string `json:"name"`
			} `json:"owner"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return 0, 0, "", "", "", err
	}

	if result.Code != 0 {
		return 0, 0, "", "", "", fmt.Errorf("API error code: %d", result.Code)
	}

	return result.Data.Aid, result.Data.Cid, result.Data.Title, result.Data.Owner.Name, result.Data.Pic, nil
}

func (d *BilibiliDownloader) setCommonHeaders(req *http.Request) {
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36")
	req.Header.Set("Referer", "https://www.bilibili.com")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Origin", "https://www.bilibili.com")

	if d.cookies != "" {
		req.Header.Set("Cookie", d.cookies)
	}
}

func (d *BilibiliDownloader) getVideoURLs(ctx context.Context, aid, cid int, bvid, preferredQuality string) (videoURL, audioURL string, qualities []string, selectedQuality string, err error) {
	qnMap := map[string]int{
		"4k":    120,
		"1080p": 80,
		"720p":  64,
		"480p":  32,
	}

	qn := qnMap[preferredQuality]
	if qn == 0 {
		qn = 127
	}

	apiURL := fmt.Sprintf("https://api.bilibili.com/x/player/playurl?avid=%d&cid=%d&qn=%d&fnval=16&fnver=0&fourk=1", aid, cid, qn)

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return "", "", nil, "", err
	}

	d.setCommonHeaders(req)
	req.Header.Set("Referer", fmt.Sprintf("https://www.bilibili.com/video/%s", bvid))

	resp, err := d.client.Do(req)
	if err != nil {
		return "", "", nil, "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", nil, "", err
	}

	var result struct {
		Code int `json:"code"`
		Data struct {
			Quality         int    `json:"quality"`
			AcceptQuality   []int  `json:"accept_quality"`
			AcceptDescription []string `json:"accept_description"`
			Dash *struct {
				Video []struct {
					ID       int    `json:"id"`
					BaseURL  string `json:"baseUrl"`
					Width    int    `json:"width"`
					Height   int    `json:"height"`
					MimeType string `json:"mimeType"`
				} `json:"video"`
				Audio []struct {
					ID       int    `json:"id"`
					BaseURL  string `json:"baseUrl"`
					MimeType string `json:"mimeType"`
				} `json:"audio"`
			} `json:"dash"`
			Durl []struct {
				URL string `json:"url"`
			} `json:"durl"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return "", "", nil, "", err
	}

	if result.Code != 0 {
		return "", "", nil, "", fmt.Errorf("API error code: %d", result.Code)
	}

	if len(result.Data.Durl) > 0 {
		qualityNames := map[int]string{
			120: "4k",
			116: "1080p60",
			112: "1080p+",
			80:  "1080p",
			74:  "720p60",
			64:  "720p",
			48:  "720p",
			32:  "480p",
			16:  "360p",
		}

		qualityName := qualityNames[result.Data.Quality]
		if qualityName == "" {
			qualityName = fmt.Sprintf("%dp", result.Data.Quality)
		}

		for _, q := range result.Data.AcceptDescription {
			qualities = append(qualities, q)
		}

		if len(qualities) == 0 {
			qualities = append(qualities, qualityName)
		}

		return result.Data.Durl[0].URL, "", qualities, qualityName, nil
	}

	if result.Data.Dash != nil && len(result.Data.Dash.Video) > 0 {
		qualityNames := map[int]string{
			120: "4k",
			116: "1080p60",
			112: "1080p+",
			80:  "1080p",
			74:  "720p60",
			64:  "720p",
			48:  "720p",
			32:  "480p",
			16:  "360p",
		}

		type VideoQuality struct {
			ID       int
			Name     string
			VideoURL string
			Width    int
			Height   int
		}

		var videoList []VideoQuality
		seenIDs := make(map[int]bool)

		for _, v := range result.Data.Dash.Video {
			if seenIDs[v.ID] {
				continue
			}
			seenIDs[v.ID] = true

			name := qualityNames[v.ID]
			if name == "" {
				name = fmt.Sprintf("%dp", v.Height)
			}

			videoList = append(videoList, VideoQuality{
				ID:       v.ID,
				Name:     name,
				VideoURL: v.BaseURL,
				Width:    v.Width,
				Height:   v.Height,
			})
		}

		if len(videoList) == 0 {
			return "", "", nil, "", fmt.Errorf("no video stream found")
		}

		for _, v := range videoList {
			qualities = append(qualities, v.Name)
		}

		selectedVideo := videoList[0]
		preferredQn := qnMap[preferredQuality]

		if preferredQn > 0 {
			for _, v := range videoList {
				if v.ID >= preferredQn {
					selectedVideo = v
					break
				}
			}
		}

		var audioStr string
		if len(result.Data.Dash.Audio) > 0 {
			audioStr = result.Data.Dash.Audio[0].BaseURL
		}

		return selectedVideo.VideoURL, audioStr, qualities, selectedVideo.Name, nil
	}

	return "", "", nil, "", fmt.Errorf("no video stream found")
}

func (d *BilibiliDownloader) DownloadVideo(ctx context.Context, videoURL string, cookies map[string]string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", videoURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36")
	req.Header.Set("Referer", "https://www.bilibili.com")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Encoding", "identity")
	req.Header.Set("Connection", "keep-alive")

	for name, value := range cookies {
		req.AddCookie(&http.Cookie{Name: name, Value: value})
	}

	client := &http.Client{Timeout: 300 * time.Second}
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
