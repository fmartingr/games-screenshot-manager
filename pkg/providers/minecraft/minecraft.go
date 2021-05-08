package minecraft

import (
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/fmartingr/games-screenshot-manager/pkg/helpers"
	"github.com/fmartingr/games-screenshot-manager/pkg/providers"
)

func getScreenshotsFromPath(game *providers.Game, path string) {
	path = helpers.ExpandUser(path)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		files, err := ioutil.ReadDir(path)
		if err != nil {
			log.Fatal(err)
		}

		for _, file := range files {
			if strings.Contains(file.Name(), ".png") {
				game.Screenshots = append(game.Screenshots, providers.Screenshot{Path: path + "/" + file.Name(), DestinationName: file.Name()})
			}
		}
	}
}

func GetGames(cliOptions providers.ProviderOptions) []providers.Game {
	var result []providers.Game
	// Standalone minecraft
	minecraftStandalone := providers.Game{Name: "Minecraft", Platform: "PC", Notes: "Standalone"}

	if runtime.GOOS == "linux" {
		getScreenshotsFromPath(&minecraftStandalone, "~/.minecraft/screenshots")

		// Flatpak minecraft
		minecraftFlatpak := providers.Game{Name: "Minecraft", Platform: "PC", Notes: "Flatpak"}
		for _, path := range [2]string{"~/.var/app/com.mojang.Minecraft/.minecraft/screenshots", "~/.var/app/com.mojang.Minecraft/data/minecraft/screenshots"} {
			getScreenshotsFromPath(&minecraftFlatpak, path)
		}
		result = append(result, minecraftFlatpak)
	} else if runtime.GOOS == "windows" {
		getScreenshotsFromPath(&minecraftStandalone, filepath.Join(os.Getenv("APPDATA"), ".minecraft/screenshots"))
	} else if runtime.GOOS == "darwin" {
		getScreenshotsFromPath(&minecraftStandalone, filepath.Join(helpers.ExpandUser("~/Library/Application Support/minecraft/screenshots")))
	}
	result = append(result, minecraftStandalone)

	return result
}
