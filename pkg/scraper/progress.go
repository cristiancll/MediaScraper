package scraper

import (
	"fmt"
	HLSDownloader "github.com/cristiancll/HLSDownloader/pkg"
	"github.com/vbauerster/mpb/v8"
	"github.com/vbauerster/mpb/v8/decor"
	"sync"
	"time"
)

type ProgressManager struct {
	start    time.Time
	progress *mpb.Progress
	bar      *mpb.Bar
}

func NewProgressManager(wg *sync.WaitGroup, total int64) *ProgressManager {
	pm := &ProgressManager{}
	pm.start = time.Now()
	pm.progress = mpb.New(mpb.WithWaitGroup(wg))
	pm.bar = pm.progress.New(total, barStyle(), totalBarOptions()...)
	return pm
}

type Progress struct {
	bar   *mpb.Bar
	start time.Time
}

type HlsBarAdapter struct {
	bar *HLSDownloader.BarUpdater
}

type BarAdapter interface {
	Increment()
	SetTotal(total int)
	Complete()
}

func (pb *Progress) Increment() {
	pb.bar.EwmaIncrement(time.Since(pb.start))
}

func (pb *Progress) SetTotal(total int) {
	pb.bar.SetTotal(int64(total), false)
}

func (pb *Progress) Complete() {
	pb.bar.Abort(true)
	pb.bar.Wait()
}

func barStyle() mpb.BarStyleComposer {
	return mpb.BarStyle().Lbound("").Filler("█").Padding("░").Tip("").Refiller("").Rbound("")
}

func totalBarOptions() []mpb.BarOption {
	return []mpb.BarOption{
		mpb.BarPriority(0),
		mpb.BarWidth(100),
		mpb.PrependDecorators(
			decor.Name(fmt.Sprintf("Medias: ")),
			decor.Name("Downloading...", decor.WCSyncSpaceR),
			decor.CountersNoUnit("%d / %d", decor.WCSyncWidth),
		),
		mpb.AppendDecorators(
			decor.OnComplete(decor.Percentage(decor.WC{W: 5}), "Done!"),
		),
	}
}

func downloadOptions(fileName string, priority int) []mpb.BarOption {
	return []mpb.BarOption{
		mpb.BarPriority(priority),
		mpb.BarWidth(65),
		mpb.BarRemoveOnComplete(),
		mpb.PrependDecorators(
			decor.Name(fmt.Sprintf("\t%s: ", fileName)),
			decor.Name("Downloading...", decor.WCSyncSpaceR),
			decor.CountersNoUnit("%d / %d", decor.WCSyncWidth),
		),
		mpb.AppendDecorators(
			decor.OnComplete(decor.Percentage(decor.WC{W: 5}), "Done!"),
		),
	}
}

func parsingOptions() []mpb.BarOption {
	return []mpb.BarOption{
		mpb.BarPriority(0),
		mpb.BarWidth(100),
		mpb.BarRemoveOnComplete(),
		mpb.PrependDecorators(
			decor.Name(fmt.Sprintf("Parsing URLs: ")),
			decor.CountersNoUnit("%d / %d", decor.WCSyncWidth),
		),
		mpb.AppendDecorators(
			decor.OnComplete(decor.Percentage(decor.WC{W: 5}), "Done!"),
		),
	}
}
