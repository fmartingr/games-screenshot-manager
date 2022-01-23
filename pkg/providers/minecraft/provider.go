package minecraft

import (
	"os"
	"path/filepath"
	"runtime"

	"github.com/fmartingr/games-screenshot-manager/internal/models"
	"github.com/fmartingr/games-screenshot-manager/pkg/helpers"
)

const Name = "minecraft"

type MinecraftProvider struct{}

func (p *MinecraftProvider) FindGames(options models.ProviderOptions) ([]*models.Game, error) {
	var result []*models.Game
	// Standalone minecraft
	minecraftStandalone := models.Game{Name: "Minecraft", Platform: "PC", Notes: "Standalone"}

	if runtime.GOOS == "linux" {
		getScreenshotsFromPath(&minecraftStandalone, "~/.minecraft/screenshots")

		// Flatpak minecraft
		minecraftFlatpak := models.Game{Name: "Minecraft", Platform: "PC", Notes: "Flatpak"}
		for _, path := range [2]string{"~/.var/app/com.mojang.Minecraft/.minecraft/screenshots", "~/.var/app/com.mojang.Minecraft/data/minecraft/screenshots"} {
			getScreenshotsFromPath(&minecraftFlatpak, path)
		}
		result = append(result, &minecraftFlatpak)
	} else if runtime.GOOS == "windows" {
		getScreenshotsFromPath(&minecraftStandalone, filepath.Join(os.Getenv("APPDATA"), ".minecraft/screenshots"))
	} else if runtime.GOOS == "darwin" {
		getScreenshotsFromPath(&minecraftStandalone, filepath.Join(helpers.ExpandUser("~/Library/Application Support/minecraft/screenshots")))
	}
	result = append(result, &minecraftStandalone)

	return result, nil
}

func NewMinecraftProvider() *MinecraftProvider {
	return &MinecraftProvider{}
}
