clean:
	rm -rf build

build: clean
	go build -o build/games-screenshot-manager cmd/games-screenshot-manager/*.go
