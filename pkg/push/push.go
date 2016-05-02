package dkrpush

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"golang.org/x/net/context"
	"golang.org/x/oauth2/google"

	"github.com/docker/distribution/digest"
	"github.com/docker/distribution/manifest"
	"github.com/docker/libtrust"
	"github.com/fsouza/go-dockerclient"
	"github.com/heroku/docker-registry-client/registry"
)

type manifestEntry struct {
	Config   string
	RepoTags []string
	Layers   []string
}

func Push(src io.Reader) error {
	manifestEntries, images, layers, err := extractTar(src)
	if err != nil {
		return err
	}

	key, err := libtrust.GenerateECP256PrivateKey()
	if err != nil {
		return err
	}

	for _, manifestEntry := range manifestEntries {
		for _, fullTag := range manifestEntry.RepoTags {
			reg, repo, tag := splitRepoTag(fullTag)
			regURL := registryNameToURL(reg)
			username, password, err := getRegCreds(regURL)
			if err != nil {
				return err
			}

			hub := &registry.Registry{
				URL:  regURL,
				Logf: registry.Quiet,
				// Logf: registry.Log,
				Client: &http.Client{
					Transport: wrapTransport(http.DefaultTransport, regURL, username, password),
				},
			}

			fmt.Fprintf(os.Stderr, "Pushing %s/%s:%s\n", reg, repo, tag)

			if err := hub.Ping(); err != nil {
				return err
			}

			fsLayers, err := uploadLayers(hub, repo, manifestEntry, layers)
			if err != nil {
				return err
			}

			mani := &manifest.Manifest{
				Versioned: manifest.Versioned{
					SchemaVersion: 1,
				},
				Name:     repo,
				Tag:      tag,
				FSLayers: fsLayers,
				History: []manifest.History{
					{V1Compatibility: string(images[strings.TrimSuffix(manifestEntry.Config, ".json")])},
				},
			}

			signedManifest, err := manifest.Sign(mani, key)
			if err != nil {
				return err
			}

			err = hub.PutManifest(mani.Name, mani.Tag, signedManifest)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func uploadLayers(hub *registry.Registry, repoName string, manifestEntry manifestEntry, layers map[string][]byte) ([]manifest.FSLayer, error) {
	var fsLayers []manifest.FSLayer

	for _, layerPath := range manifestEntry.Layers {
		var (
			layerID   = strings.TrimSuffix(layerPath, "/layer.tar")
			layerData = layers[layerID]
			zdata     []byte
		)

		{
			var zbuf bytes.Buffer
			zw := gzip.NewWriter(&zbuf)
			io.Copy(zw, bytes.NewReader(layerData))
			zw.Close()
			zdata = zbuf.Bytes()
		}

		layerDigest, err := digest.FromBytes(zdata)
		if err != nil {
			return nil, err
		}

		exists, err := hub.HasLayer(repoName, layerDigest)
		if err != nil {
			return nil, err
		}
		if exists {
			fmt.Fprintf(os.Stderr, "Existing layer  %s\n", layerID)
			fsLayers = append(fsLayers, manifest.FSLayer{layerDigest})
			continue
		}

		fmt.Fprintf(os.Stderr, "Uploading layer %s\n", layerID)
		err = hub.UploadLayer(repoName, layerDigest, bytes.NewReader(zdata))
		if err != nil {
			return nil, err
		}

		fsLayers = append(fsLayers, manifest.FSLayer{layerDigest})
	}

	return fsLayers, nil
}

func extractTar(src io.Reader) (manifest []manifestEntry, images, layers map[string][]byte, err error) {
	layers = map[string][]byte{}
	images = map[string][]byte{}

	r := tar.NewReader(src)
	for {
		hdr, err := r.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, nil, nil, err
		}

		if len(hdr.Name) == 64+len("/layer.tar") && strings.HasSuffix(hdr.Name, "/layer.tar") {
			layerID := strings.TrimSuffix(hdr.Name, "/layer.tar")

			data, err := ioutil.ReadAll(r)
			if err != nil {
				return nil, nil, nil, err
			}
			layers[layerID] = data
		}

		if len(hdr.Name) == 64+5 && strings.HasSuffix(hdr.Name, ".json") {
			imageID := strings.TrimSuffix(hdr.Name, ".json")

			data, err := ioutil.ReadAll(r)
			if err != nil {
				return nil, nil, nil, err
			}
			images[imageID] = data
		}

		if hdr.Name == "manifest.json" {
			data, err := ioutil.ReadAll(r)
			if err != nil {
				return nil, nil, nil, err
			}
			err = json.Unmarshal(data, &manifest)
			if err != nil {
				return nil, nil, nil, err
			}
		}

		io.Copy(ioutil.Discard, r)
	}

	return manifest, images, layers, nil
}

func splitRepoTag(t string) (registry, repo, tag string) {

	parts := strings.SplitN(t, "/", 3)
	if len(parts) == 1 {
		parts = []string{
			"docker.io",
			"library/",
			parts[0],
		}
	}
	if len(parts) == 2 {
		parts = []string{
			"docker.io",
			parts[0],
			parts[1],
		}
	}

	tagParts := strings.SplitN(parts[2], ":", 2)
	if len(tagParts) == 1 {
		parts = []string{
			parts[0],
			parts[1],
			tagParts[0],
			"latest",
		}
	}
	if len(tagParts) == 2 {
		parts = []string{
			parts[0],
			parts[1],
			tagParts[0],
			tagParts[1],
		}
	}

	registry = parts[0]
	repo = strings.Join(parts[1:3], "/")
	tag = parts[3]
	return
}

func registryNameToURL(name string) string {
	if name == "docker.io" {
		return "https://index.docker.io"
	}
	return "https://" + name
}

func getRegCreds(url string) (username, password string, err error) {
	if os.Getenv("DKR_USERNAME") != "" {
		return os.Getenv("DKR_USERNAME"), os.Getenv("DKR_PASSWORD"), nil
	}

	if strings.Contains(url, "gcr.io") {
		ts, err := google.DefaultTokenSource(context.Background())
		if err != nil {
			return "", "", err
		}

		token, err := ts.Token()
		if err != nil {
			return "", "", err
		}

		return "_token", token.AccessToken, nil
	}

	if os.Getenv("HOME") != "" {
		auth, err := docker.NewAuthConfigurationsFromDockerCfg()
		if err != nil {
			return "", "", err
		}

		creds, f := auth.Configs[url]
		if f {
			return creds.Username, creds.Password, nil
		}

		if url == "https://index.docker.io" {
			creds, f := auth.Configs["https://index.docker.io/v1/"]
			if f {
				return creds.Username, creds.Password, nil
			}
		}
	}

	return "", "", errors.New("not logged in")
}

func wrapTransport(transport http.RoundTripper, url, username, password string) http.RoundTripper {
	tokenTransport := &registry.TokenTransport{
		Transport: transport,
		Username:  username,
		Password:  password,
	}
	basicAuthTransport := &registry.BasicTransport{
		Transport: tokenTransport,
		URL:       url,
		Username:  username,
		Password:  password,
	}
	errorTransport := &registry.ErrorTransport{
		Transport: basicAuthTransport,
	}
	return errorTransport
}
