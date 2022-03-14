package exif

import (
	"fmt"
	"log"

	"github.com/barasher/go-exiftool"
)

func GetTags(path string) (map[string]string, error) {
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
