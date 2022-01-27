package processor

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/fmartingr/games-screenshot-manager/internal/models"
	"github.com/fmartingr/games-screenshot-manager/pkg/helpers"
	"github.com/gosimple/slug"
	"github.com/sirupsen/logrus"
)

type Processor struct {
	logger  *logrus.Entry
	options models.Options

	games chan *models.Game
	wg    *sync.WaitGroup
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
	p.logger.Debug("Worker started")
	for {
		select {
		case <-ctx.Done():
			return

		case game := <-p.games:
			if err := p.processGame(game); err != nil {
				p.logger.Errorf("Error processing game %s from %s: %s", game.Name, game.Provider, err)
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

	p.logger.WithFields(logrus.Fields{
		"provider": game.Provider,
		"name":     game.Name,
	}).Debugf("Processing game")

	// Do not continue if there's no screenshots
	if len(game.Screenshots) == 0 {
		return
	}

	destinationPath := filepath.Join(helpers.ExpandUser(p.options.OutputPath), game.Platform)
	if len(game.Name) > 0 {
		destinationPath = filepath.Join(destinationPath, game.Name)
	} else {
		p.logger.Warnf("found game with ID: %s from %s without a name", game.ID, game.Provider)
		destinationPath = filepath.Join(destinationPath, game.ID)
	}

	// Check if folder exists (create otherwise)
	if _, err := os.Stat(destinationPath); os.IsNotExist(err) && !p.options.DryRun {
		mkdirErr := os.MkdirAll(destinationPath, 0711)
		if mkdirErr != nil {
			p.logger.Errorf("Couldn't create directory with name %s, falling back to %s", game.Name, slug.Make(game.Name))
			destinationPath = filepath.Join(helpers.ExpandUser(p.options.OutputPath), game.Platform, slug.Make(game.Name))
			os.MkdirAll(destinationPath, 0711)
		}
	}

	if p.options.DownloadCovers && !p.options.DryRun && game.CoverURL != "" {
		destinationCoverPath := filepath.Join(destinationPath, ".cover")
		coverPath, err := helpers.DownloadURLIntoTempFile(game.CoverURL)
		if err != nil {
			p.logger.Errorf("Error donwloading cover for game %s from %s: %s", game.Name, game.Provider, err)
		} else {
			if _, err := os.Stat(destinationCoverPath); os.IsNotExist(err) {
				helpers.CopyFile(coverPath, destinationCoverPath)
			}
		}

	}

	for _, screenshot := range game.Screenshots {
		destinationPath := filepath.Join(destinationPath, screenshot.GetDestinationName())

		if _, err := os.Stat(destinationPath); !os.IsNotExist(err) {
			sourceMd5, err := helpers.Md5File(screenshot.Path)
			if err != nil {
				p.logger.Errorf("Can't get hash of source file for game %s from %s: %s", game.Name, game.Provider, err)
				return err
			}
			destinationMd5, err := helpers.Md5File(destinationPath)
			if err != nil {
				p.logger.Errorf("Can't get hash of destination file for game %s from %s: %s", game.Name, game.Provider, err)
				return err
			}

			if !bytes.Equal(sourceMd5, destinationMd5) {
				// Images are not equal, we should copy it anyway, but how?
				p.logger.Warnf("Found different screenshot with equal timestamp for game %s from %s on %s", game.Name, game.Provider, screenshot.Path)
			}

		} else {
			if p.options.DryRun {
				p.logger.Infof("cp %s %s", filepath.Base(screenshot.Path), strings.Replace(destinationPath, helpers.ExpandUser(p.options.OutputPath), "", 1))
			} else {
				if _, err := helpers.CopyFile(screenshot.Path, destinationPath); err != nil {
					p.logger.WithFields(logrus.Fields{
						"src":  screenshot.Path,
						"dest": destinationPath,
					}).Errorf("Error during copy operation: %s", err)
				}
			}
		}
	}

	return nil
}

func NewProcessor(logger *logrus.Logger, options models.Options) *Processor {
	return &Processor{
		logger:  logger.WithField("from", "processor"),
		games:   make(chan *models.Game, options.ProcessBufferSize),
		options: options,
		wg:      &sync.WaitGroup{},
	}
}
