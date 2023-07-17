package main

import (
	"fmt"
	"mediaScraper/pkg/scraper"
)

func main() {
	var URLs []string
	for i := 100; i < 200; i++ {
		fileName := fmt.Sprintf("https://via.placeholder.com/%dx%d", i, i)
		URLs = append(URLs, fileName)
	}
	sc, err := scraper.New(URLs, "", 10)
	if err != nil {
		panic(err)
	}
	sc.Scrape()
}
