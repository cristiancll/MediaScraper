package scraper

import (
	HLSDownloader "github.com/cristiancll/HLSDownloader/pkg"
	"io"
	"net/http"
	"net/url"
	"os"
)

type MediaType string

const (
	Video MediaType = "video"
	Image           = "image"
)

type Download struct {
	url      url.URL
	path     string
	fileName string
	ext      string
	mType    MediaType
	isM3U8   bool
	bar      BarAdapter
}

func NewDownload(url url.URL, path string, mType MediaType, isM3U8 bool) *Download {
	return &Download{
		url:    url,
		path:   path,
		mType:  mType,
		isM3U8: isM3U8,
	}
}

func (d *Download) Start(bar Progress) error {
	d.bar = &bar
	if d.isM3U8 {
		return d.downloadM3U8()
	}
	return d.downloadFile()
}

func (d *Download) downloadFile() error {
	defer d.bar.Complete()
	resp, err := http.Get(d.url.String())
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	out, err := os.Create(d.path)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}

	d.bar.Increment()
	return nil
}

func (d *Download) downloadM3U8() error {
	hls, err := HLSDownloader.New(d.url.String(), d.path)
	if err != nil {
		return err
	}
	err = hls.SetWorkers(20)
	if err != nil {
		return err
	}

	err = hls.SetBar(d.bar)
	if err != nil {
		return err
	}
	_, err = hls.Download()
	if err != nil {
		return err
	}
	return nil
}
