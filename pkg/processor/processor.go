package processor

import (
	"bytes"
	"context"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/fmartingr/games-screenshot-manager/internal/models"
	"github.com/fmartingr/games-screenshot-manager/pkg/helpers"
	"github.com/gosimple/slug"
)

type Processor struct {
	games   chan *models.Game
	options models.Options
	wg      *sync.WaitGroup
}

func (p *Processor) Start(ctx context.Context) {
	for i := 0; i < p.options.WorkersNum; i++ {
		go p.process(ctx)
	}
}

func (p *Processor) Process(game *models.Game) {
	p.wg.Add(1)
	p.games <- game
}

func (p *Processor) process(ctx context.Context) {
	log.Println("Started worker process")
	for {
		select {
		case <-ctx.Done():
			return

		case game := <-p.games:
			if err := p.processGame(game); err != nil {
				log.Printf("[err] %s", err)
			}
		}
	}
}

func (p *Processor) Wait() {
	p.wg.Wait()
}

// TODO: Reduce into smaller functions
func (p *Processor) processGame(game *models.Game) (err error) {
	defer p.wg.Done()

	log.Printf("Processing game: %s", game.Name)

	// Do not continue if there's no screenshots
	if len(game.Screenshots) == 0 {
		return
	}

	destinationPath := filepath.Join(helpers.ExpandUser(p.options.OutputPath), game.Platform)
	if len(game.Name) > 0 {
		destinationPath = filepath.Join(destinationPath, game.Name)
	} else {
		log.Printf("[IMPORTANT] Game ID %s has no name!", game.ID)
		destinationPath = filepath.Join(destinationPath, game.ID)
	}

	// Check if folder exists (create otherwise)
	if _, err := os.Stat(destinationPath); os.IsNotExist(err) && !p.options.DryRun {
		mkdirErr := os.MkdirAll(destinationPath, 0711)
		if mkdirErr != nil {
			log.Printf("[ERROR] Couldn't create directory with name %s, falling back to %s", game.Name, slug.Make(game.Name))
			destinationPath = filepath.Join(helpers.ExpandUser(p.options.OutputPath), game.Platform, slug.Make(game.Name))
			os.MkdirAll(destinationPath, 0711)
		}
	}

	if p.options.DownloadCovers && !p.options.DryRun && game.CoverURL != "" {
		destinationCoverPath := filepath.Join(destinationPath, ".cover")
		coverPath, err := helpers.DownloadURLIntoTempFile(game.CoverURL)
		if err != nil {
			log.Printf("[error] Error donwloading cover: %s", err)
		}

		if _, err := os.Stat(destinationCoverPath); os.IsNotExist(err) {
			helpers.CopyFile(coverPath, destinationCoverPath)
		}
	}

	for _, screenshot := range game.Screenshots {
		destinationPath := filepath.Join(destinationPath, screenshot.GetDestinationName())

		if _, err := os.Stat(destinationPath); !os.IsNotExist(err) {
			sourceMd5, err := helpers.Md5File(screenshot.Path)
			if err != nil {
				log.Fatal(err)
				return err
			}
			destinationMd5, err := helpers.Md5File(destinationPath)
			if err != nil {
				log.Fatal(err)
				return err
			}

			if !bytes.Equal(sourceMd5, destinationMd5) {
				// Images are not equal, we should copy it anyway, but how?
				log.Println("Found different screenshot with equal timestamp for game ", game.Name, screenshot.Path)
			}

		} else {
			if p.options.DryRun {
				log.Println(filepath.Base(screenshot.Path), " -> ", strings.Replace(destinationPath, helpers.ExpandUser(p.options.OutputPath), "", 1))
			} else {
				if _, err := helpers.CopyFile(screenshot.Path, destinationPath); err != nil {
					log.Printf("[error] error during operation: %s", err)
				}
			}
		}
	}

	return nil
}

func NewProcessor(options models.Options) *Processor {
	return &Processor{
		games:   make(chan *models.Game, options.ProcessBufferSize),
		options: options,
		wg:      &sync.WaitGroup{},
	}
}
