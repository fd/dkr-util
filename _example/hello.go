package main

import (
	"io"
	"log"
	"net/http"
	"os"
)

// curl https://mkcert.org/generate/ > etc/ssl/certs/ca-certificates.crt

func main() {
	res, err := http.Get("https://letsencrypt.status.io/")
	if err != nil {
		log.Fatal(err)
	}
	io.Copy(os.Stdout, res.Body)
}
