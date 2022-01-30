package minecraft

import (
	"os"
	"path/filepath"
	"runtime"

	"github.com/fmartingr/games-screenshot-manager/internal/models"
	"github.com/fmartingr/games-screenshot-manager/pkg/helpers"
	"github.com/sirupsen/logrus"
)

const Name = "minecraft"

type MinecraftProvider struct {
	logger *logrus.Entry
}

func (p *MinecraftProvider) FindGames(options models.ProviderOptions) ([]*models.Game, error) {
	var result []*models.Game
	// Standalone minecraft
	minecraftStandalone := models.Game{Name: "Minecraft", Platform: "PC", Notes: "Standalone"}

	if runtime.GOOS == "linux" {
		if err := getScreenshotsFromPath(&minecraftStandalone, "~/.minecraft/screenshots"); err != nil {
			p.logger.Error(err)
		}

		// Flatpak minecraft
		minecraftFlatpak := models.Game{Name: "Minecraft", Platform: "PC", Notes: "Flatpak"}
		for _, path := range [2]string{"~/.var/app/com.mojang.Minecraft/.minecraft/screenshots", "~/.var/app/com.mojang.Minecraft/data/minecraft/screenshots"} {
			if err := getScreenshotsFromPath(&minecraftFlatpak, path); err != nil {
				p.logger.Error(err)
			}
		}
		result = append(result, &minecraftFlatpak)
	} else if runtime.GOOS == "windows" {
		if err := getScreenshotsFromPath(&minecraftStandalone, filepath.Join(os.Getenv("APPDATA"), ".minecraft/screenshots")); err != nil {
			p.logger.Error(err)
		}
	} else if runtime.GOOS == "darwin" {
		if err := getScreenshotsFromPath(&minecraftStandalone, filepath.Join(helpers.ExpandUser("~/Library/Application Support/minecraft/screenshots"))); err != nil {
			p.logger.Error(err)
		}
	}
	result = append(result, &minecraftStandalone)

	return result, nil
}

func NewMinecraftProvider(logger *logrus.Logger, cache models.Cache) models.Provider {
	return &MinecraftProvider{
		logger: logger.WithField("from", "provider."+Name),
	}
}
