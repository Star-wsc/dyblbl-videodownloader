package downloader

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"time"

	"github.com/Star-wsc/dyblbl-videodownloader/internal/models"

	"github.com/Eyevinn/mp4ff/mp4"
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

// DownloadVideo 下载视频文件（保留原方法兼容性）
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

// DownloadToFile 下载视频到文件
func (d *BilibiliDownloader) DownloadToFile(ctx context.Context, videoURL, filePath string) error {
	req, err := http.NewRequestWithContext(ctx, "GET", videoURL, nil)
	if err != nil {
		return fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36")
	req.Header.Set("Referer", "https://www.bilibili.com")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Encoding", "identity")
	req.Header.Set("Connection", "keep-alive")

	if d.cookies != "" {
		req.Header.Set("Cookie", d.cookies)
	}

	client := &http.Client{Timeout: 300 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("HTTP错误: %d, 响应: %s", resp.StatusCode, string(body))
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType == "text/html" || contentType == "" {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("意外的内容类型: %s, 响应: %s", contentType, string(body))
	}

	out, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("创建文件失败: %w", err)
	}
	defer out.Close()

	buf := make([]byte, 32*1024)
	written, err := io.CopyBuffer(out, resp.Body, buf)
	if err != nil {
		return fmt.Errorf("写入文件失败: %w", err)
	}

	fmt.Printf("[Bilibili] 下载完成: %d 字节 -> %s\n", written, filePath)
	return nil
}

// DownloadWithMerge 下载视频和音频并合并
func (d *BilibiliDownloader) DownloadWithMerge(ctx context.Context, videoURL, audioURL, outputPath string) error {
	fmt.Printf("[Bilibili] 开始下载\n")
	fmt.Printf("[Bilibili] 视频URL: %s\n", truncateURL(videoURL, 80))
	fmt.Printf("[Bilibili] 音频URL: %s\n", truncateURL(audioURL, 80))
	fmt.Printf("[Bilibili] 输出路径: %s\n", outputPath)

	if audioURL == "" {
		fmt.Printf("[Bilibili] 无音频URL，仅下载视频\n")
		return d.DownloadToFile(ctx, videoURL, outputPath)
	}

	tempDir := filepath.Dir(outputPath)
	videoTemp := filepath.Join(tempDir, fmt.Sprintf("temp_video_%d.m4s", time.Now().UnixNano()))
	audioTemp := filepath.Join(tempDir, fmt.Sprintf("temp_audio_%d.m4s", time.Now().UnixNano()))

	fmt.Printf("[Bilibili] 临时视频路径: %s\n", videoTemp)
	fmt.Printf("[Bilibili] 临时音频路径: %s\n", audioTemp)

	defer func() {
		os.Remove(videoTemp)
		os.Remove(audioTemp)
	}()

	fmt.Printf("[Bilibili] 下载视频流...\n")
	if err := d.DownloadToFile(ctx, videoURL, videoTemp); err != nil {
		return fmt.Errorf("下载视频流失败: %w", err)
	}

	videoInfo, _ := os.Stat(videoTemp)
	fmt.Printf("[Bilibili] 视频下载完成: %d 字节\n", videoInfo.Size())

	fmt.Printf("[Bilibili] 下载音频流...\n")
	if err := d.DownloadToFile(ctx, audioURL, audioTemp); err != nil {
		fmt.Printf("[Bilibili] 警告: 下载音频流失败: %v\n", err)
		if err := os.Rename(videoTemp, outputPath); err != nil {
			return fmt.Errorf("重命名视频文件失败: %w", err)
		}
		return nil
	}

	audioInfo, _ := os.Stat(audioTemp)
	fmt.Printf("[Bilibili] 音频下载完成: %d 字节\n", audioInfo.Size())

	fmt.Printf("[Bilibili] 移除M4S头部...\n")
	videoClean := videoTemp + ".clean.mp4"
	audioClean := audioTemp + ".clean.m4a"

	defer func() {
		os.Remove(videoClean)
		os.Remove(audioClean)
	}()

	if err := removeM4SHeader(videoTemp, videoClean); err != nil {
		fmt.Printf("[Bilibili] 警告: 移除视频M4S头部失败: %v\n", err)
		videoClean = videoTemp
	}
	if err := removeM4SHeader(audioTemp, audioClean); err != nil {
		fmt.Printf("[Bilibili] 警告: 移除音频M4S头部失败: %v\n", err)
		audioClean = audioTemp
	}

	fmt.Printf("[Bilibili] 合并视频和音频...\n")
	if err := MergeMP4(videoClean, audioClean, outputPath); err != nil {
		fmt.Printf("[Bilibili] 合并失败: %v\n", err)
		if err := os.Rename(videoClean, outputPath); err != nil {
			return fmt.Errorf("重命名视频文件失败: %w", err)
		}
	}

	return nil
}

func removeM4SHeader(inputPath, outputPath string) error {
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return err
	}

	if len(data) > 8 {
		allZero := true
		for i := 0; i < 8; i++ {
			if data[i] != 0 {
				allZero = false
				break
			}
		}
		if allZero {
			return os.WriteFile(outputPath, data[8:], 0644)
		}
	}

	return os.WriteFile(outputPath, data, 0644)
}

func truncateURL(url string, maxLen int) string {
	if len(url) <= maxLen {
		return url
	}
	return url[:maxLen] + "..."
}

// MergeMP4 合并视频和音频文件
func MergeMP4(videoPath, audioPath, outputPath string) error {
	videoData, err := os.ReadFile(videoPath)
	if err != nil {
		return fmt.Errorf("读取视频文件失败: %w", err)
	}

	audioData, err := os.ReadFile(audioPath)
	if err != nil {
		return fmt.Errorf("读取音频文件失败: %w", err)
	}

	fmt.Printf("[Merge] 原始视频大小: %d 字节, 音频大小: %d 字节\n", len(videoData), len(audioData))

	if len(videoData) > 8 {
		allZero := true
		for i := 0; i < 8; i++ {
			if videoData[i] != 0 {
				allZero = false
				break
			}
		}
		if allZero {
			fmt.Printf("[Merge] 检测到Bilibili m4s格式，移除视频8字节头部\n")
			videoData = videoData[8:]
		}
	}

	if len(audioData) > 8 {
		allZero := true
		for i := 0; i < 8; i++ {
			if audioData[i] != 0 {
				allZero = false
				break
			}
		}
		if allZero {
			fmt.Printf("[Merge] 检测到Bilibili m4s格式，移除音频8字节头部\n")
			audioData = audioData[8:]
		}
	}

	fmt.Printf("[Merge] 移除头部后 - 视频: %d 字节, 音频: %d 字节\n", len(videoData), len(audioData))

	tempVideoPath := videoPath + ".clean.mp4"
	tempAudioPath := audioPath + ".clean.m4a"

	if err := os.WriteFile(tempVideoPath, videoData, 0644); err != nil {
		return fmt.Errorf("写入临时视频文件失败: %w", err)
	}
	defer os.Remove(tempVideoPath)

	if err := os.WriteFile(tempAudioPath, audioData, 0644); err != nil {
		return fmt.Errorf("写入临时音频文件失败: %w", err)
	}
	defer os.Remove(tempAudioPath)

	ffmpegPath, err := getBuiltinFFmpeg()
	if err != nil {
		fmt.Printf("[Merge] 未找到内置FFmpeg，尝试纯Go合并: %v\n", err)
		return mergeWithMP4FF(videoData, audioData, outputPath)
	}

	fmt.Printf("[Merge] 使用内置FFmpeg: %s\n", ffmpegPath)

	cmd := exec.Command(ffmpegPath,
		"-i", tempVideoPath,
		"-i", tempAudioPath,
		"-c:v", "copy",
		"-c:a", "copy",
		"-y",
		outputPath,
	)
	setSysProcAttr(cmd)

	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("[Merge] FFmpeg错误输出: %s\n", string(output))
		return fmt.Errorf("FFmpeg合并失败: %w", err)
	}

	if info, err := os.Stat(outputPath); err == nil {
		fmt.Printf("[Merge] FFmpeg合并成功! 大小: %d 字节\n", info.Size())
	}

	return nil
}

func getBuiltinFFmpeg() (string, error) {
	exePath, err := os.Executable()
	if err != nil {
		return "", err
	}
	exeDir := filepath.Dir(exePath)

	var ffmpegNames []string
	switch runtime.GOOS {
	case "windows":
		ffmpegNames = []string{"ffmpeg-windows-amd64.exe", "ffmpeg.exe"}
	case "linux":
		if runtime.GOARCH == "arm64" {
			ffmpegNames = []string{"ffmpeg-linux-arm64", "ffmpeg"}
		} else {
			ffmpegNames = []string{"ffmpeg-linux-amd64", "ffmpeg"}
		}
	case "darwin":
		ffmpegNames = []string{"ffmpeg-darwin-arm64", "ffmpeg-darwin-amd64", "ffmpeg"}
	default:
		return "", fmt.Errorf("unsupported platform: %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	for _, name := range ffmpegNames {
		searchPaths := []string{
			filepath.Join(exeDir, "ffmpeg", name),
			filepath.Join(exeDir, name),
			filepath.Join(".", "ffmpeg", name),
			filepath.Join(".", name),
		}

		for _, path := range searchPaths {
			if info, err := os.Stat(path); err == nil && !info.IsDir() {
				if runtime.GOOS != "windows" {
					os.Chmod(path, 0755)
				}
				return path, nil
			}
		}
	}

	return "", fmt.Errorf("built-in FFmpeg not found for %s/%s", runtime.GOOS, runtime.GOARCH)
}

func mergeWithMP4FF(videoData, audioData []byte, outputPath string) error {
	videoReader := bytes.NewReader(videoData)
	audioReader := bytes.NewReader(audioData)

	videoFile, err := mp4.DecodeFile(videoReader)
	if err != nil {
		return fmt.Errorf("解析视频MP4失败: %w", err)
	}

	audioFile, err := mp4.DecodeFile(audioReader)
	if err != nil {
		return fmt.Errorf("解析音频MP4失败: %w", err)
	}

	fmt.Printf("[Merge] 视频 - 段数: %d, Init: %v, Moov: %v\n",
		len(videoFile.Segments), videoFile.Init != nil, videoFile.Moov != nil)
	fmt.Printf("[Merge] 音频 - 段数: %d, Init: %v, Moov: %v\n",
		len(audioFile.Segments), audioFile.Init != nil, audioFile.Moov != nil)

	if videoFile.Init != nil && audioFile.Init != nil {
		return mergeInitSegments(videoFile, audioFile, outputPath)
	}

	if videoFile.Moov != nil && audioFile.Moov != nil {
		return mergeProgressiveFiles(videoFile, audioFile, outputPath)
	}

	return fmt.Errorf("无法识别的MP4格式，请安装FFmpeg到ffmpeg目录")
}

func mergeInitSegments(videoFile, audioFile *mp4.File, outputPath string) error {
	outFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("创建输出文件失败: %w", err)
	}
	defer outFile.Close()

	videoInit := videoFile.Init
	audioInit := audioFile.Init

	if videoInit == nil || audioInit == nil {
		return fmt.Errorf("缺少init segment")
	}

	fmt.Printf("[Merge] 视频init轨道数: %d, 音频init轨道数: %d\n",
		len(videoInit.Moov.Traks), len(audioInit.Moov.Traks))

	if videoInit.Ftyp != nil {
		if err := videoInit.Ftyp.Encode(outFile); err != nil {
			return fmt.Errorf("编码ftyp失败: %w", err)
		}
	}

	for _, trak := range audioInit.Moov.Traks {
		videoInit.Moov.AddChild(trak)
	}

	if err := videoInit.Moov.Encode(outFile); err != nil {
		return fmt.Errorf("编码moov失败: %w", err)
	}

	for i, seg := range videoFile.Segments {
		fmt.Printf("[Merge] 编码视频段 %d\n", i)
		if err := seg.Encode(outFile); err != nil {
			return fmt.Errorf("编码视频segment失败: %w", err)
		}
	}

	for i, seg := range audioFile.Segments {
		fmt.Printf("[Merge] 编码音频段 %d\n", i)
		if err := seg.Encode(outFile); err != nil {
			return fmt.Errorf("编码音频segment失败: %w", err)
		}
	}

	if info, err := os.Stat(outputPath); err == nil {
		fmt.Printf("[Merge] 合并成功! 大小: %d 字节\n", info.Size())
	}

	return nil
}

func mergeProgressiveFiles(videoFile, audioFile *mp4.File, outputPath string) error {
	outFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("创建输出文件失败: %w", err)
	}
	defer outFile.Close()

	videoMoov := videoFile.Moov
	audioMoov := audioFile.Moov

	if videoMoov == nil || audioMoov == nil {
		return fmt.Errorf("缺少moov box")
	}

	fmt.Printf("[Merge] 视频轨道数: %d, 音频轨道数: %d\n",
		len(videoMoov.Traks), len(audioMoov.Traks))

	if videoFile.Ftyp != nil {
		if err := videoFile.Ftyp.Encode(outFile); err != nil {
			return fmt.Errorf("编码ftyp失败: %w", err)
		}
	}

	for _, trak := range audioMoov.Traks {
		videoMoov.AddChild(trak)
	}

	if err := videoMoov.Encode(outFile); err != nil {
		return fmt.Errorf("编码moov失败: %w", err)
	}

	if videoFile.Mdat != nil {
		if err := videoFile.Mdat.Encode(outFile); err != nil {
			return fmt.Errorf("编码视频mdat失败: %w", err)
		}
	}

	if audioFile.Mdat != nil {
		if err := audioFile.Mdat.Encode(outFile); err != nil {
			return fmt.Errorf("编码音频mdat失败: %w", err)
		}
	}

	fmt.Printf("[Merge] 渐进式MP4合并成功!\n")
	return nil
}

type MP4Box struct {
	Type string
	Size uint64
	Data []byte
}

func readAllBoxes(r *bytes.Reader) ([]MP4Box, error) {
	var boxes []MP4Box

	for {
		if r.Len() < 8 {
			break
		}

		var header [8]byte
		if _, err := r.Read(header[:]); err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}

		size := binary.BigEndian.Uint32(header[0:4])
		boxType := string(header[4:8])

		var boxSize uint64
		var dataOffset uint64 = 8

		if size == 1 {
			var extHeader [8]byte
			if _, err := r.Read(extHeader[:]); err != nil {
				break
			}
			boxSize = binary.BigEndian.Uint64(extHeader[:])
			dataOffset = 16
		} else if size == 0 {
			boxSize = uint64(r.Len()) + dataOffset
		} else {
			boxSize = uint64(size)
		}

		if boxSize < dataOffset {
			break
		}

		dataSize := boxSize - dataOffset
		data := make([]byte, dataSize)
		if _, err := r.Read(data); err != nil && err != io.EOF {
			return nil, err
		}

		boxes = append(boxes, MP4Box{
			Type: boxType,
			Size: boxSize,
			Data: data,
		})
	}

	return boxes, nil
}
