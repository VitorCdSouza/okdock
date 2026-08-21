package system

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

type Info struct {
	MemoryTotal     int64   `json:"memoryTotal"`
	MemoryAvailable int64   `json:"memoryAvailable"`
	MemoryUsed      int64   `json:"memoryUsed"`
	DiskTotal       int64   `json:"diskTotal"`
	DiskFree        int64   `json:"diskFree"`
	DiskUsed        int64   `json:"diskUsed"`
	CPUCount        int     `json:"cpuCount"`
	CPUPercent      float64 `json:"cpuPercent"`
}

type Reader interface {
	Read(diskPath string) (Info, error)
}

type ProcReader struct {
	mu   sync.Mutex
	last *cpuSample
}

type cpuSample struct {
	idle  uint64
	total uint64
	at    time.Time
}

func (r *ProcReader) Read(diskPath string) (Info, error) {
	var info Info

	mem, err := readMeminfo()
	if err != nil {
		return info, err
	}
	info.MemoryTotal = mem["MemTotal"]
	info.MemoryAvailable = mem["MemAvailable"]
	info.MemoryUsed = info.MemoryTotal - info.MemoryAvailable

	var st syscall.Statfs_t
	if err := syscall.Statfs(diskPath, &st); err == nil {
		bs := int64(st.Bsize)
		info.DiskTotal = int64(st.Blocks) * bs
		info.DiskFree = int64(st.Bavail) * bs
		info.DiskUsed = info.DiskTotal - info.DiskFree
	}

	info.CPUCount, info.CPUPercent = r.cpu()
	return info, nil
}

func (r *ProcReader) cpu() (count int, percent float64) {
	idle, total, n, err := readStat()
	if err != nil {
		return 0, 0
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	prev := r.last
	r.last = &cpuSample{idle: idle, total: total, at: time.Now()}
	if prev == nil || total <= prev.total {
		return n, 0
	}
	dTotal := float64(total - prev.total)
	dIdle := float64(idle - prev.idle)
	return n, clamp((dTotal-dIdle)/dTotal*100, 0, 100)
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func readMeminfo() (map[string]int64, error) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return nil, fmt.Errorf("lendo /proc/meminfo: %w", err)
	}
	defer f.Close()

	out := map[string]int64{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		key, rest, ok := strings.Cut(sc.Text(), ":")
		if !ok {
			continue
		}
		fields := strings.Fields(rest)
		if len(fields) == 0 {
			continue
		}
		n, err := strconv.ParseInt(fields[0], 10, 64)
		if err != nil {
			continue
		}
		out[key] = n * 1024
	}
	return out, sc.Err()
}

func readStat() (idle, total uint64, cpuCount int, err error) {
	f, err := os.Open("/proc/stat")
	if err != nil {
		return 0, 0, 0, err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) == 0 {
			continue
		}
		switch {
		case fields[0] == "cpu":
			for i, v := range fields[1:] {
				n, err := strconv.ParseUint(v, 10, 64)
				if err != nil {
					continue
				}
				total += n
				if i == 3 || i == 4 {
					idle += n
				}
			}
		case strings.HasPrefix(fields[0], "cpu"):
			cpuCount++
		}
	}
	return idle, total, cpuCount, sc.Err()
}

type StaticReader struct{ Info Info }

func (s StaticReader) Read(string) (Info, error) { return s.Info, nil }
