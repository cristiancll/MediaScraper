package scraper

import (
	"fmt"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

func checkURL(url url.URL) (fileName string, mType MediaType, isM3U8 bool, err error) {
	resp, err := http.Get(url.String())
	if err != nil {
		return
	}
	if resp.StatusCode != 200 {
		err = fmt.Errorf("status code error: %s - %s", resp.Status, url.String())
		return
	}
	mediaType, _, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if err != nil {
		return
	}
	mediaTypeSplit := strings.Split(mediaType, "/")
	ext := mediaTypeSplit[1]
	switch mediaTypeSplit[0] {
	case "video":
		mType = Video
	case "image":
		mType = Image
	case "application":
		if checkIfM3U8(mediaType) {
			mType = Video
			isM3U8 = true
			ext = "ts"
		}
	default:
		err = fmt.Errorf("unsupported media type: %s", mediaType)
		return
	}
	fileName = getFileName(url, resp, ext)
	return
}

func getFileName(URL url.URL, resp *http.Response, ext string) string {
	fileName := func(name string, ext string) string {
		nameExt := strings.ToLower(filepath.Ext(name))
		if nameExt != "" && nameExt != "m3u8" {
			return name
		}
		return fmt.Sprintf("%s.%s", name, ext)
	}

	// Method 1: Get filename from Content-Disposition header, if present
	contentDisposition := resp.Header.Get("Content-Disposition")
	if contentDisposition != "" {
		_, params, err := mime.ParseMediaType(contentDisposition)
		if err == nil {
			if name, ok := params["filename"]; ok {
				return fileName(name, ext)
			}
		}
	}

	// Method 2: Get filename from URL path
	if name := path.Base(URL.Path); name != "" {
		return fileName(name, ext)
	}

	// Method 3: Get filename from URL query parameter, if present
	queryParams, err := url.ParseQuery(URL.RawQuery)
	if err == nil {
		if name := queryParams.Get("filename"); name != "" {
			return fileName(name, ext)
		}
	}

	// Method 4: Get filename from URL fragment, if present
	if fragment := URL.Fragment; fragment != "" {
		if strings.HasPrefix(fragment, "filename=") {
			name := strings.TrimPrefix(fragment, "filename=")
			return fileName(name, ext)
		}
	}

	// Method 5: Generate a random filename
	return fmt.Sprintf("%d.%s", time.Now().Unix(), ext)
}

func checkIfM3U8(mediaType string) (ok bool) {
	hlsOptions := []string{
		"application/x-mpegURL",
		"application/vnd.apple.mpegurl",
		"application/mpegurl",
	}
	if contains(hlsOptions, mediaType) {
		return true
	}
	return false
}
func contains(options []string, choice string) bool {
	for _, option := range options {
		if option == choice {
			return true
		}
	}
	return false
}

func createDirIfNotExist(dirPath string) error {
	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		err = os.MkdirAll(dirPath, 0755)
		if err != nil {
			return err
		}
	}
	return nil
}

func checkFileDownloaded(download *Download) bool {
	if _, err := os.Stat(download.path); os.IsNotExist(err) {
		return false
	}
	return true
}

func getWD() string {
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}
	return wd
}

func durationHumanized(duration time.Duration) string {
	return time.Time{}.Add(duration).Format("15:04:05")
}
