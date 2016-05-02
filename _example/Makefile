

build:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o ./rootfs/bin/hello ./hello.go
	tar -C ./rootfs -c . | dkr package -o hello.tar

load: build
	cat hello.tar | docker load

push: build
	dkr push -i hello.tar

pull-certs:
	mkdir -p rootfs/etc/ssl/certs
	curl https://mkcert.org/generate/ > ./rootfs/etc/ssl/certs/ca-certificates.crt
