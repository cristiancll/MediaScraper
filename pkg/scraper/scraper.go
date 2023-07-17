package scraper

import (
	"errors"
	"fmt"
	"github.com/dustin/go-humanize"
	"github.com/vbauerster/mpb/v8"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

type Scraper struct {
	totalInitialURLs int
	rawURLs          []string
	output           string
	workers          int

	mu        sync.Mutex
	downloads []*Download
	fileNames Set
	errors    []error

	pManager *ProgressManager
	stats    *Statistics
}

type Set map[string]struct{}

const MaxDuplicateAttempts = 1000

type Statistics struct {
	TotalExisting  *int64
	TotalDownloads *int64
	TotalSize      *int64
	TotalTime      time.Duration
	TotalErrors    *int64
}

func New(rawURLs []string, output string, workers int) (*Scraper, error) {
	err := createDirIfNotExist(output)
	if err != nil {
		return nil, err
	}
	return &Scraper{
		totalInitialURLs: len(rawURLs),
		rawURLs:          rawURLs,
		output:           output,
		workers:          workers,
		fileNames:        Set{},
		stats: &Statistics{
			TotalExisting:  new(int64),
			TotalDownloads: new(int64),
			TotalSize:      new(int64),
			TotalTime:      0,
			TotalErrors:    new(int64),
		},
	}, nil
}

func (s *Scraper) ensureUnique(fileName string) (string, error) {
	if fileName == "" {
		return "", errors.New("invalid file name")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := 0; i < MaxDuplicateAttempts; i++ {
		uFileName := fileName
		if i > 0 {
			ext := filepath.Ext(fileName)
			name := fileName[:len(fileName)-len(ext)]
			uFileName = fmt.Sprintf("%s (%d)%s", name, i, ext)
		}
		_, ok := s.fileNames[uFileName]
		if !ok {
			filePath := filepath.Join(s.output, uFileName)
			_, err := os.Stat(filePath)
			if err != nil {
				s.fileNames[uFileName] = struct{}{}
				return uFileName, nil
			}
		}
	}
	return "", errors.New("could not find unique file name, too many duplicates")
}

func (s *Scraper) parseURLs() {
	totalURLs := len(s.rawURLs)
	wg := sync.WaitGroup{}
	progress := mpb.New(mpb.WithWaitGroup(&wg))
	start := time.Now()
	bar := progress.New(int64(totalURLs), barStyle(), parsingOptions()...)

	for i := 0; i < s.workers; i++ {
		wg.Add(1)
		go func(bar *mpb.Bar, i int) {
			defer wg.Done()
			for {
				rawURL, ok := s.nextURL()
				if !ok {
					break
				}
				if rawURL == "" {
					s.totalInitialURLs--
					bar.EwmaIncrement(time.Since(start))
					continue
				}
				URL, err := url.Parse(rawURL)
				if err != nil {
					s.logError(err)
					bar.EwmaIncrement(time.Since(start))
					continue
				}
				fileName, mType, isM3U8, err := checkURL(*URL)
				if err != nil {
					s.logError(err)
					bar.EwmaIncrement(time.Since(start))
					continue
				}
				fileName, err = s.ensureUnique(fileName)
				if err != nil {
					s.logError(err)
					bar.EwmaIncrement(time.Since(start))
					continue
				}
				path := filepath.Join(s.output, fileName)
				download := NewDownload(*URL, path, mType, isM3U8)
				s.parseURL(download)
				bar.EwmaIncrement(time.Since(start))
			}
		}(bar, i)
	}
	progress.Wait()
}

func (s *Scraper) Scrape() {
	s.parseURLs()
	totalDownloads := len(s.downloads)
	if totalDownloads == 0 {
		s.showStats()
		return
	}
	wg := sync.WaitGroup{}
	s.pManager = NewProgressManager(&wg, int64(totalDownloads))
	wg.Add(1)
	go func() {
		defer wg.Done()
		s.start()
	}()
	s.pManager.progress.Wait()
	s.stats.TotalTime = time.Since(s.pManager.start)
	s.showStats()
}

func (s *Scraper) showStats() {

	fmt.Println("Statistics:")
	fmt.Printf("Total URLs: %d\n", s.totalInitialURLs)
	if *s.stats.TotalExisting > 0 {
		fmt.Printf("\t%d existing\n", *s.stats.TotalExisting)
	}
	if *s.stats.TotalDownloads > 0 {
		fmt.Printf("\t%d downloaded\n", *s.stats.TotalDownloads)
		fmt.Printf("\t\t%s size\n", humanize.Bytes(uint64(*s.stats.TotalSize)))
		fmt.Printf("\t\t%s time\n", durationHumanized(s.stats.TotalTime))
	}
	if *s.stats.TotalErrors > 0 {
		fmt.Printf("Total failed: %d\n", *s.stats.TotalErrors)
		if len(s.errors) > 0 {
			fmt.Println("Errors:")
			for _, err := range s.errors {
				fmt.Println(err)
			}
		}
	}
}

func (s *Scraper) nextURL() (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.rawURLs) == 0 {
		return "", false
	}
	URL := s.rawURLs[0]
	s.rawURLs = s.rawURLs[1:]
	return URL, true
}

func (s *Scraper) nextDownload() (*Download, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.downloads) == 0 {
		return nil, false
	}
	download := s.downloads[0]
	s.downloads = s.downloads[1:]
	return download, true
}

func (s *Scraper) logError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.errors = append(s.errors, err)
	atomic.AddInt64(s.stats.TotalErrors, 1)
}

func (s *Scraper) parseURL(download *Download) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.downloads = append(s.downloads, download)
}

func (s *Scraper) start() {
	wg := sync.WaitGroup{}
	for i := 0; i < s.workers; i++ {
		wg.Add(1)
		go func(priority int) {
			defer wg.Done()
			s.scrapeMedia(priority)
		}(i + 1)
	}
	wg.Wait()
}

func (s *Scraper) scrapeMedia(priority int) {
	for {
		download, ok := s.nextDownload()
		if !ok {
			return
		}
		if !checkFileDownloaded(download) {
			fileName := filepath.Base(download.path)
			var total int64 = 1
			if download.isM3U8 {
				total = 0
			}
			bar := s.pManager.progress.New(total, barStyle(), downloadOptions(fileName, priority)...)
			progress := Progress{
				bar:   bar,
				start: time.Now(),
			}
			err := download.Start(progress)
			if err != nil {
				s.logError(err)
				download.bar.Complete()
			} else {
				atomic.AddInt64(s.stats.TotalDownloads, 1)
				info, err := os.Stat(download.path)
				if err == nil {
					size := info.Size()
					if size > 0 {
						atomic.AddInt64(s.stats.TotalSize, size)
					}
				}
			}
		} else {
			atomic.AddInt64(s.stats.TotalExisting, 1)
		}
		s.pManager.bar.EwmaIncrement(time.Since(s.pManager.start))
	}
}
