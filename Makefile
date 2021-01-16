build:
	which go || curl -L https://git.io/vQhTU | bash
	go mod download
	go build
