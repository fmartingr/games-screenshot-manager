package games

import (
	"log"
	"os"
	"path"
)

const DatetimeFormat = "2006-01-02_15-04-05"

type Game struct {
	ID          string
	Name        string
	Platform    string
	Provider    string
	Screenshots []Screenshot
	Notes       string
}

type Screenshot struct {
	Path            string
	DestinationName string
}

func (screenshot Screenshot) GetDestinationName() string {
	if screenshot.DestinationName != "" {
		return screenshot.DestinationName
	}
	fileStat, statErr := os.Stat(screenshot.Path)
	if statErr != nil {
		log.Fatal(statErr)
	}
	return fileStat.ModTime().Format(DatetimeFormat) + path.Ext(screenshot.Path)
}
