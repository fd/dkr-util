package dkrpackage

import (
	"archive/tar"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"io/ioutil"
	"path"
	"strings"
	"time"
)

func Package(dst io.Writer, src io.Reader) error {
	layerTar, conf, err := mkLayerTar(src)
	if err != nil {
		return err
	}

	imageConf, err := mkImageConfig(conf)
	if err != nil {
		return err
	}

	manifest, err := mkManifest(conf)
	if err != nil {
		return err
	}

	out, err := mkImageArchive(conf, manifest, imageConf, layerTar)
	if err != nil {
		return err
	}

	_, err = io.Copy(dst, bytes.NewReader(out))
	if err != nil {
		return err
	}

	return nil
}

type Config struct {
	RepoTags     []string         `json:"repo_tags"`
	Author       string           `json:"author"`
	Architecture string           `json:"architecture"`
	OS           string           `json:"os"`
	Config       *ContainerConfig `json:"config"`

	diffID    string
	imageID   string
	imageTime time.Time
}

type ContainerConfig struct {
	User         string
	Memory       int64
	MemorySwap   int64
	CPUShares    int64 `json:"CpuShares"`
	ExposedPorts map[string]struct{}
	Env          []string
	Entrypoint   []string
	Cmd          []string
	Volumes      []string
	WorkingDir   string
}

type imageConfig struct {
	Created      time.Time        `json:"created"`
	Author       string           `json:"author"`
	Architecture string           `json:"architecture"`
	OS           string           `json:"os"`
	Config       *ContainerConfig `json:"config"`
	RootFS       rootFSConfig     `json:"rootfs"`
	History      []historyEntry   `json:"history"`
}

type rootFSConfig struct {
	DiffIDs []string `json:"diff_ids"`
	Type    string   `json:"type"`
}

type historyEntry struct {
	Created    time.Time `json:"created"`
	Author     string    `json:"author,omitempty"`
	CreatedBy  string    `json:"created_by"`
	Comment    string    `json:"comment,omitempty"`
	EmptyLayer bool      `json:"empty_layer,omitempty"`
}

type manifestEntry struct {
	Config   string
	RepoTags []string
	Layers   []string
}

var ftime = time.Date(1988, time.February, 1, 0, 0, 0, 0, time.UTC)

func mkLayerTar(src io.Reader) ([]byte, *Config, error) {
	var (
		confData  []byte
		tarBuf    bytes.Buffer
		conf      *Config
		imageTime = ftime
		r         = tar.NewReader(src)
		w         = tar.NewWriter(&tarBuf)
	)

	for {
		hdr, err := r.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, nil, err
		}

		atime := hdr.AccessTime
		ctime := hdr.ChangeTime
		mtime := hdr.ModTime

		hdr.AccessTime = ftime
		hdr.ChangeTime = ftime
		hdr.ModTime = ftime
		hdr.Name = strings.TrimPrefix(path.Join("/", hdr.Name), "/")
		if hdr.FileInfo().IsDir() {
			hdr.Name += "/"
		}
		if hdr.Linkname != "" {
			hdr.Linkname = strings.TrimPrefix(path.Join("/", hdr.Linkname), "/")
		}
		if hdr.Name == "/" {
			io.Copy(ioutil.Discard, r)
			continue
		}
		if strings.HasPrefix(path.Base(hdr.Name), "._") {
			io.Copy(ioutil.Discard, r)
			continue
		}
		hdr.Gid = 1
		hdr.Uid = 1
		hdr.Gname = "root"
		hdr.Uname = "root"
		hdr.Xattrs = nil

		if atime.After(imageTime) {
			imageTime = atime
		}
		if ctime.After(imageTime) {
			imageTime = ctime
		}
		if mtime.After(imageTime) {
			imageTime = mtime
		}

		if hdr.Name == ".docker.json" {
			confData, err = ioutil.ReadAll(r)
			if err != nil {
				return nil, nil, err
			}
			continue
		}

		err = w.WriteHeader(hdr)
		if err != nil {
			return nil, nil, err
		}

		_, err = io.Copy(w, r)
		if err != nil {
			return nil, nil, err
		}
	}

	err := w.Close()
	if err != nil {
		return nil, nil, err
	}

	if len(confData) > 0 {
		err := json.Unmarshal(confData, &conf)
		if err != nil {
			return nil, nil, err
		}
	}

	if conf == nil {
		conf = &Config{}
	}

	layerSum := sha256.Sum256(tarBuf.Bytes())
	diffID := hex.EncodeToString(layerSum[:])
	conf.diffID = diffID
	conf.imageTime = imageTime

	return tarBuf.Bytes(), conf, nil
}

func mkImageConfig(conf *Config) ([]byte, error) {
	iconf := &imageConfig{
		Created:      conf.imageTime,
		Author:       conf.Author,
		Architecture: conf.Architecture,
		OS:           conf.OS,
		Config:       conf.Config,

		RootFS: rootFSConfig{
			Type:    "layers",
			DiffIDs: []string{"sha256:" + conf.diffID},
		},

		History: []historyEntry{
			{
				Author:    conf.Author,
				Created:   conf.imageTime,
				CreatedBy: "/bin/sh -c #(nop) dkr-package",
				Comment:   "Created by dkr-package",
			},
		},
	}

	if iconf.OS == "" {
		iconf.OS = "linux"
	}
	if iconf.Architecture == "" {
		iconf.Architecture = "amd64"
	}

	data, err := json.Marshal(&iconf)
	if err != nil {
		return nil, err
	}

	imageSum := sha256.Sum256(data)
	imageHex := hex.EncodeToString(imageSum[:])
	conf.imageID = imageHex

	return data, nil
}

func mkManifest(conf *Config) ([]byte, error) {
	manifest := []manifestEntry{
		{
			Config:   conf.imageID + ".json",
			RepoTags: conf.RepoTags,
			Layers:   []string{conf.diffID + "/layer.tar"},
		},
	}

	return json.Marshal(&manifest)
}

func mkImageArchive(conf *Config, manifest, imageConf, layerTar []byte) ([]byte, error) {
	var (
		tarBuf bytes.Buffer
		w      = tar.NewWriter(&tarBuf)
		err    error
	)

	err = w.WriteHeader(&tar.Header{
		Name:     "manifest.json",
		Typeflag: tar.TypeReg,
		Mode:     0644,
		Size:     int64(len(manifest)),
	})
	if err != nil {
		return nil, err
	}
	_, err = io.Copy(w, bytes.NewReader(manifest))
	if err != nil {
		return nil, err
	}

	err = w.WriteHeader(&tar.Header{
		Name:     conf.imageID + ".json",
		Typeflag: tar.TypeReg,
		Mode:     0644,
		Size:     int64(len(imageConf)),
	})
	if err != nil {
		return nil, err
	}
	_, err = io.Copy(w, bytes.NewReader(imageConf))
	if err != nil {
		return nil, err
	}

	err = w.WriteHeader(&tar.Header{
		Name:     conf.diffID + "/layer.tar",
		Typeflag: tar.TypeReg,
		Mode:     0644,
		Size:     int64(len(layerTar)),
	})
	if err != nil {
		return nil, err
	}
	_, err = io.Copy(w, bytes.NewReader(layerTar))
	if err != nil {
		return nil, err
	}

	err = w.WriteHeader(&tar.Header{
		Name:     conf.diffID + "/VERSION",
		Typeflag: tar.TypeReg,
		Mode:     0644,
		Size:     3,
	})
	if err != nil {
		return nil, err
	}
	_, err = io.Copy(w, strings.NewReader("1.0"))
	if err != nil {
		return nil, err
	}

	err = w.WriteHeader(&tar.Header{
		Name:     conf.diffID + "/json",
		Typeflag: tar.TypeReg,
		Mode:     0644,
		Size:     2,
	})
	if err != nil {
		return nil, err
	}
	_, err = io.Copy(w, strings.NewReader("{}"))
	if err != nil {
		return nil, err
	}

	err = w.Close()
	if err != nil {
		return nil, err
	}

	return tarBuf.Bytes(), nil
}
