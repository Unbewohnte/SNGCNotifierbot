all: clean
	mkdir -p bin
	go build -o SNGCNOTIFIERbot ./cmd && mv SNGCNOTIFIERbot bin/ ; cp config.json bin/

clean:
	rm -rf bin/ SNGCNOTIFIERbot*


cross: clean
	mkdir -p bin/SNGCNOTIFIERbot_linux_amd64 && \
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/SNGCNOTIFIERbot_linux_amd64/SNGCNOTIFIERbot_linux ./cmd && \
	cp COPYING README.md bin/SNGCNOTIFIERbot_linux_amd64/ 

	mkdir -p bin/SNGCNOTIFIERbot_windows_amd64 && \
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o bin/SNGCNOTIFIERbot_windows_amd64/SNGCNOTIFIERbot_windows.exe ./cmd  && \
	cp COPYING README.md bin/SNGCNOTIFIERbot_windows_amd64/ 

	mkdir -p bin/SNGCNOTIFIERbot_darwin_amd64 && \
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -o bin/SNGCNOTIFIERbot_darwin_amd64/SNGCNOTIFIERbot_darwin ./cmd && \
	cp COPYING README.md bin/SNGCNOTIFIERbot_darwin_amd64/ 
