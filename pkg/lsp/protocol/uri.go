package protocol

import (
	"net/url"
	"path/filepath"
	"runtime"
	"strings"
)

// PathToURI converts a filesystem path to a file:// URI.
// Relative paths are resolved against the current working directory.
func PathToURI(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = filepath.Clean(path)
	}
	return filePathToURI(abs)
}

// NormalizeFileURI returns uriOrPath unchanged if it already uses the file:// scheme,
// otherwise converts it with PathToURI.
func NormalizeFileURI(uriOrPath string) string {
	if strings.HasPrefix(uriOrPath, "file://") {
		return uriOrPath
	}
	return PathToURI(uriOrPath)
}

func filePathToURI(path string) string {
	path = filepath.Clean(path)
	if runtime.GOOS == "windows" {
		path = strings.ReplaceAll(path, "\\", "/")
		if len(path) >= 2 && path[1] == ':' {
			drive := strings.ToLower(string(path[0]))
			path = "/" + drive + ":" + path[2:]
		}
	}
	u := url.URL{Scheme: "file", Path: path}
	return u.String()
}
