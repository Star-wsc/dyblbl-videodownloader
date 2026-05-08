# vdl - Video Downloader CLI

命令行视频下载工具，支持抖音和B站视频下载。

## 下载

从 [Releases](https://github.com/Star-wsc/dyblbl-videodownloader/releases) 页面下载对应平台的二进制文件。

```bash
# 下载后添加执行权限
chmod +x vdl-linux-amd64

# 建议移动到 PATH 目录
sudo mv vdl-linux-amd64 /usr/local/bin/vdl
```

## 使用方法

```bash
# 下载抖音视频
vdl douyin "https://v.douyin.com/xxxxx"

# 下载B站视频
vdl bilibili "https://www.bilibili.com/video/BVxxxxxx"

# 指定输出路径
vdl douyin "https://v.douyin.com/xxxxx" -o /path/to/video.mp4

# 指定画质
vdl douyin "https://v.douyin.com/xxxxx" -q super
vdl bilibili "https://www.bilibili.com/video/BVxxxxxx" -q 1080p
```

## 配置

```bash
# 查看当前配置
vdl config show

# 设置下载目录
vdl config set download-dir /data/videos

# 设置代理
vdl config set proxy http://127.0.0.1:7890

# 设置B站Cookie（用于下载高画质视频）
vdl config set bilibili-cookie "SESSDATA=xxxx; bili_jct=xxxx"

# 设置抖音Cookie
vdl config set douyin-cookie "你的cookie"
```

## 画质选项

| 平台 | 可用画质 |
|------|----------|
| 抖音 | normal, high, super |
| B站 | 480p, 720p, 1080p, 4k |

## 作为 OpenClaw/Hermes 工具使用

```bash
# 示例：在脚本中调用
vdl douyin "https://v.douyin.com/xxxxx" -o /tmp/video.mp4

# 下载完成后可继续处理
ffmpeg -i /tmp/video.mp4 -vf "scale=640:480" /tmp/output.mp4
```

## 编译

```bash
# Linux
GOOS=linux GOARCH=amd64 go build -o vdl-linux-amd64 .

# macOS
GOOS=darwin GOARCH=arm64 go build -o vdl-darwin-arm64 .

# Windows (交叉编译)
GOOS=windows GOARCH=amd64 go build -o vdl-windows-amd64.exe .
```

## License

MIT
