package minecraft

import (
	"io/ioutil"
	"log"
	"os"
	"path"
	"runtime"
	"strings"

	"github.com/fmartingr/games-screenshot-mananger/pkg/games"
	"github.com/fmartingr/games-screenshot-mananger/pkg/helpers"
)

func getScreenshotsFromPath(game *games.Game, path string) {
	path = helpers.ExpandUser(path)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		files, err := ioutil.ReadDir(path)
		if err != nil {
			log.Fatal(err)
		}

		for _, file := range files {
			if strings.Contains(file.Name(), ".png") {
				game.Screenshots = append(game.Screenshots, games.Screenshot{Path: path + "/" + file.Name(), DestinationName: file.Name()})
			}
		}
	}
}

func GetGames() []games.Game {
	var result []games.Game
	// Standalone minecraft
	minecraftStandalone := games.Game{Name: "Minecraft", Platform: "PC", Notes: "Standalone"}

	if runtime.GOOS == "linux" {
		getScreenshotsFromPath(&minecraftStandalone, "~/.minecraft/screenshots")

		// Flatpak minecraft
		minecraftFlatpak := games.Game{Name: "Minecraft", Platform: "PC", Notes: "Flatpak"}
		getScreenshotsFromPath(&minecraftFlatpak, "~/.var/app/com.mojang.Minecraft/.minecraft/screenshots")
		result = append(result, minecraftFlatpak)
	} else if runtime.GOOS == "windows" {
		getScreenshotsFromPath(&minecraftStandalone, path.Join(os.Getenv("APPDATA"), ".minecraft/screenshots"))
	} else if runtime.GOOS == "darwin" {
		getScreenshotsFromPath(&minecraftStandalone, path.Join(helpers.ExpandUser("~/Library/Application Support/minecraft/screenshots")))
	}
	result = append(result, minecraftStandalone)

	return result
}
