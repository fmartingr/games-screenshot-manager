module github.com/fmartingr/games-screenshot-manager

go 1.23.3

toolchain go1.24.1

require (
	git.nakama.town/fmartingr/gotoolkit v0.1.0
	github.com/barasher/go-exiftool v1.10.0
	github.com/gosimple/slug v1.14.0
	github.com/lmittmann/tint v1.0.7
	github.com/sirupsen/logrus v1.9.3
	github.com/stretchr/testify v1.10.0
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/gosimple/unidecode v1.0.1 // indirect
	github.com/pelletier/go-toml/v2 v2.2.3 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	golang.org/x/sys v0.22.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace git.nakama.town/fmartingr/gotoolkit => ../gotoolkit
