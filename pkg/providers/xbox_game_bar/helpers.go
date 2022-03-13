package xbox_game_bar

import (
	"fmt"
	"log"
	"os"
	"strings"

	exiftool "github.com/barasher/go-exiftool"
	"github.com/rwcarlsen/goexif/exif"
)

func getExifTagsWithOld(path string) error {
	fileDescriptor, errFileDescriptor := os.Open(path)
	if errFileDescriptor != nil {
		return fmt.Errorf("Couldn't open file %s: %s", path, errFileDescriptor)
	}

	exifData, errExifData := exif.Decode(fileDescriptor)
	if errExifData != nil {
		return fmt.Errorf("Decoding EXIF data from %s: %s", path, errExifData)
	}

	defer fileDescriptor.Close()

	t, _ := exifData.MarshalJSON()
	log.Println(t)

	titleTag := "Title"
	if strings.Contains(path, ".png") {
		titleTag = "Microsoft Game DVR Title"
	}
	title, err := exifData.Get(exif.FieldName(titleTag))
	if err != nil {
		return fmt.Errorf("Error getting tag %s from %s: %s", titleTag, path, errExifData)
	}

	log.Println(title)

	return nil
}

func getExifTags(path string) error {
	et, err := exiftool.NewExiftool()
	if err != nil {
		fmt.Printf("Error when intializing: %v\n", err)
		return nil
	}
	defer et.Close()

	fileInfos := et.ExtractMetadata(path)

	for _, fileInfo := range fileInfos {
		if fileInfo.Err != nil {
			fmt.Printf("Error concerning %v: %v\n", fileInfo.File, fileInfo.Err)
			continue
		}

		log.Println(fileInfo.GetString("MicrosoftGameDVRTitle"))
		log.Println(fileInfo.GetString("Title"))

	}
	return nil
}
