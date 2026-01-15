package covers

import (
	"embed"
	"fmt"
	"io"
	"os"
)

//go:embed *.png
var Covers embed.FS

// GetHytaleCover reads the embedded Hytale cover image and returns it as a temporary file
func GetCover(gameID string) (*os.File, error) {
	file, err := Covers.Open(fmt.Sprintf("%s.png", gameID))
	if err != nil {
		return nil, err
	}
	defer file.Close()

	tempFile, err := os.CreateTemp("", "hytale_cover_*.png")
	if err != nil {
		return nil, err
	}

	_, err = io.Copy(tempFile, file)
	if err != nil {
		tempFile.Close()
		os.Remove(tempFile.Name())
		return nil, err
	}

	// Reset file pointer to beginning
	_, err = tempFile.Seek(0, 0)
	if err != nil {
		tempFile.Close()
		os.Remove(tempFile.Name())
		return nil, err
	}

	return tempFile, nil
}
