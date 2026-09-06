package gamesscreenshotmanager

import (
	"fmt"
	"log"

	"github.com/barasher/go-exiftool"
)

func GetExifTagsWithTool(et *exiftool.Exiftool, path string) (map[string]string, error) {
	fileInfos := et.ExtractMetadata(path)

	if len(fileInfos) == 0 {
		return nil, fmt.Errorf("no metadata found for %s", path)
	}

	result := make(map[string]string, len(fileInfos[0].Fields))

	var err error
	for _, fileInfo := range fileInfos {
		if fileInfo.Err != nil {
			return nil, fmt.Errorf("error parsing file exif for %v: %v", fileInfo.File, fileInfo.Err)
		}

		for k := range fileInfo.Fields {
			result[k], err = fileInfo.GetString(k)
			if err != nil {
				log.Printf("error getting tag %s: %s", k, err)
			}
		}
	}

	return result, nil
}

func GetExifTags(path string) (map[string]string, error) {
	et, err := exiftool.NewExiftool()
	if err != nil {
		return nil, fmt.Errorf("error intializing exiftool: %v\n", err)
	}
	defer et.Close()

	fileInfos := et.ExtractMetadata(path)

	if len(fileInfos) == 0 {
		return nil, fmt.Errorf("no metadata found for %s", path)
	}

	result := make(map[string]string, len(fileInfos[0].Fields))

	for _, fileInfo := range fileInfos {
		if fileInfo.Err != nil {
			return nil, fmt.Errorf("error parsing file exif for %v: %v", fileInfo.File, fileInfo.Err)
		}

		for k := range fileInfo.Fields {
			result[k], err = fileInfo.GetString(k)
			if err != nil {
				log.Printf("error getting tag %s: %s", k, err)
			}
		}
	}

	return result, nil
}

// exiftoolRequirement is the program every provider that reads EXIF data needs.
var exiftoolRequirement = Requirement{
	Binary:  "exiftool",
	Package: "exiftool",
	Reason:  "reads the capture date from a screenshot",
}
