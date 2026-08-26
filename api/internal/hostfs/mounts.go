package hostfs

import (
	"bufio"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const mountInfo = "/proc/self/mountinfo"

// paths the kernel and docker put in every container, never a place for a bind mount
var systemPaths = []string{"/proc", "/sys", "/dev", "/run", "/var/run", "/etc"}

// BindMounts reads the mount table for the host folders bound into this process
func BindMounts() []string { return bindMountsFrom(mountInfo) }

func bindMountsFrom(file string) []string {
	f, err := os.Open(file)
	if err != nil {
		return nil
	}
	defer f.Close()

	seen := map[string]bool{}
	var out []string
	scan := bufio.NewScanner(f)
	for scan.Scan() {
		point := mountPoint(scan.Text())
		if point == "" || point == "/" || seen[point] || under(point, systemPaths) {
			continue
		}
		info, err := os.Stat(point)
		if err != nil || !info.IsDir() {
			continue
		}
		seen[point] = true
		out = append(out, point)
	}
	if err := scan.Err(); err != nil {
		return nil
	}
	sort.Strings(out)
	return out
}

// the mount point is the fifth field, and a space in it arrives as \040
func mountPoint(line string) string {
	fields := strings.Fields(line)
	if len(fields) < 5 {
		return ""
	}
	point := strings.ReplaceAll(fields[4], `\040`, " ")
	if !filepath.IsAbs(point) {
		return ""
	}
	return filepath.Clean(point)
}

func under(path string, roots []string) bool {
	for _, root := range roots {
		if path == root || strings.HasPrefix(path, root+"/") {
			return true
		}
	}
	return false
}
