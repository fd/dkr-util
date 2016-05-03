package main

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"

	"github.com/fd/dkr-util/pkg/cat"
	"github.com/fd/dkr-util/pkg/package"
	"github.com/fd/dkr-util/pkg/push"
	"gopkg.in/alecthomas/kingpin.v2"
	"limbo.services/version"
)

func main() {
	err := run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		inputTar  string
		outputTar string
	)

	app := kingpin.New("dkr", "Docker utilities").Version(version.Get().String()).Author(version.Get().ReleasedBy)

	packageCmd := app.Command("package", "Make a new image without running docker")
	packageCmd.Flag("input", "Tar archive to use").Short('i').Default("-").PlaceHolder("FILE").StringVar(&inputTar)
	packageCmd.Flag("output", "Path to output Tar archive").Short('o').Default("-").PlaceHolder("FILE").StringVar(&outputTar)

	pushCmd := app.Command("push", "Push an image archive to a registry")
	pushCmd.Flag("input", "Tar archive to use").Short('i').Default("-").PlaceHolder("FILE").StringVar(&inputTar)

	catTagsCmd := app.Command("cat-tags", "Print the tags conatined in an image archive")
	catTagsCmd.Flag("input", "Tar archive to use").Short('i').Default("-").PlaceHolder("FILE").StringVar(&inputTar)

	switch kingpin.MustParse(app.Parse(os.Args[1:])) {

	case packageCmd.FullCommand():
		r, err := openStream(inputTar)
		if err != nil {
			return err
		}

		var buf bytes.Buffer

		err = dkrpackage.Package(&buf, r)
		if err != nil {
			return err
		}

		err = putStream(outputTar, &buf)
		if err != nil {
			return err
		}

	case pushCmd.FullCommand():
		r, err := openStream(inputTar)
		if err != nil {
			return err
		}

		err = dkrpush.Push(r)
		if err != nil {
			return err
		}

	case catTagsCmd.FullCommand():
		r, err := openStream(inputTar)
		if err != nil {
			return err
		}

		err = dkrcat.Tags(r)
		if err != nil {
			return err
		}

	}

	return nil
}

const stdio = "-"

func openStream(name string) (io.Reader, error) {
	if name == stdio {
		return os.Stdin, nil
	}
	return os.Open(name)
}

func putStream(name string, buf *bytes.Buffer) error {
	if name == stdio {
		_, err := io.Copy(os.Stdout, buf)
		return err
	}
	return ioutil.WriteFile(name, buf.Bytes(), 0644)
}
