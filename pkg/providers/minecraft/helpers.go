package minecraft

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/fmartingr/games-screenshot-manager/internal/models"
	"github.com/fmartingr/games-screenshot-manager/pkg/helpers"
)

func getScreenshotsFromPath(game *models.Game, path string) error {
	path = helpers.ExpandUser(path)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		files, err := ioutil.ReadDir(path)
		if err != nil {
			return fmt.Errorf("error reading from %s: %s", path, err)
		}

		for _, file := range files {
			if strings.Contains(file.Name(), ".png") {
				game.Screenshots = append(game.Screenshots, models.Screenshot{Path: path + "/" + file.Name(), DestinationName: file.Name()})
			}
		}
	}
	return nil
}
