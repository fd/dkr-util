# dkr-util

## Usage

```
usage: dkr [<flags>] <command> [<args> ...]

Docker utilities

Flags:
  --help     Show context-sensitive help (also try --help-long and --help-man).
  --version  Show application version.

Commands:
  help [<command>...]
    Show help.


  package [<flags>]
    Make a new image without running docker

    -i, --input=FILE   Tar archive to use
    -o, --output=FILE  Path to output Tar archive

  push [<flags>]
    Push an image archive to a registry

    -i, --input=FILE  Tar archive to use

  cat-tags [<flags>]
    Print the tags conatined in an image archive

    -i, --input=FILE  Tar archive to use
```

## .docker.json format

```json5
{
  "repo_tags": ["<tag>"], // required
  "author": "<author>",
  "config": {
    "User": "<User>",
    "Memory": 123,
    "MemorySwap": 123,
    "CpuShares": 100,
    "ExposedPorts": {
      "80/tcp": {}
    },
    "Env": [
      "VAR=val",
    ],
    "Entrypoint": ["cmd", "args"],
    "Cmd": ["cmd", "args"],
    "Volumes": {
      "/data:ro": {}
    },
    "WorkingDir": "<WorkingDir>"
  }
}
```
