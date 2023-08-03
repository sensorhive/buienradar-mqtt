all: local

local:
	GOOS="linux" GOARCH="amd64" go build -o bin/buienradar-mqtt-linux-amd64 .
	GOOS="linux" GOARCH="arm64" go build -o bin/buienradar-mqtt-linux-arm64 .
containers:
	podman build --jobs=2 --platform=linux/amd64,linux/arm64 --manifest buienradar-mqtt .
#containers-publish:
#	# you need to `podman login src.tty.cat` first
#	podman manifest push localhost/buienradar-mqtt docker://src.tty.cat/home.arpa/buienradar-mqtt:latest
