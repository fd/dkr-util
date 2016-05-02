package dkrcat

import (
	"archive/tar"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"sort"
)

func Tags(src io.Reader) error {
	r := tar.NewReader(src)

	for {
		hdr, err := r.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil
		}

		if hdr.Name != "manifest.json" {
			io.Copy(ioutil.Discard, r)
			continue
		}

		data, err := ioutil.ReadAll(r)
		if err != nil {
			return err
		}

		var manifest []manifestEntry

		err = json.Unmarshal(data, &manifest)
		if err != nil {
			return err
		}

		var tags []string
		for _, e := range manifest {
			for _, t := range e.RepoTags {
				tags = append(tags, t)
			}
		}
		sort.Strings(tags)
		utags := tags[:0]
		last := ""
		for _, t := range tags {
			if last != t {
				last = t
				utags = append(utags, t)
			}
		}

		for _, t := range utags {
			fmt.Println(t)
		}
		return nil
	}

	return errors.New("no tags found")
}

type manifestEntry struct {
	Config   string
	RepoTags []string
	Layers   []string
}
