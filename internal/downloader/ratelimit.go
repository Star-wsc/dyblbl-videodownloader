package downloader

import (
	"io"
	"sync"
	"time"
)

type RateLimitReader struct {
	reader    io.Reader
	limit     int64 // bytes per second, 0=unlimited
	bucket    int64
	maxBucket int64
	lastTime  time.Time
	mu        sync.Mutex
}

func NewRateLimitReader(reader io.Reader, limitKBps int) io.Reader {
	if limitKBps <= 0 {
		return reader
	}
	limit := int64(limitKBps) * 1024
	return &RateLimitReader{
		reader:    reader,
		limit:     limit,
		bucket:    limit,
		maxBucket: limit,
		lastTime:  time.Now(),
	}
}

func (r *RateLimitReader) Read(p []byte) (int, error) {
	r.mu.Lock()

	now := time.Now()
	elapsed := now.Sub(r.lastTime)
	r.lastTime = now

	// refill bucket
	r.bucket += int64(float64(r.limit) * elapsed.Seconds())
	if r.bucket > r.maxBucket {
		r.bucket = r.maxBucket
	}

	// if bucket is empty, sleep to accumulate tokens
	if r.bucket <= 0 {
		r.mu.Unlock()
		// sleep for a small fixed interval to let tokens accumulate
		time.Sleep(50 * time.Millisecond)
		// return 0 bytes but NO error — caller will retry
		// this is safe because io.Copy handles (0, nil) by retrying,
		// and the next call will have a refilled bucket
		return 0, nil
	}

	// allow at least 1 byte per read to prevent starvation
	maxRead := r.bucket
	if maxRead < 1 {
		maxRead = 1
	}
	if int64(len(p)) > maxRead {
		p = p[:maxRead]
	}

	r.mu.Unlock()

	n, err := r.reader.Read(p)
	if n > 0 {
		r.mu.Lock()
		r.bucket -= int64(n)
		r.mu.Unlock()
	}
	return n, err
}
