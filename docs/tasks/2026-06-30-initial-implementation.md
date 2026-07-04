# Task: Initial Implementation of Devroom

In our first task we will build an initial implementation of `devroom`,
a console program for running claude (or similar) in a local container. It 
is described in:

- An initial proposal - docs/devroom-proposal.md
- An imaginary walkthrough - docs/vision.md

## Step 1: Go project skeleton

Write the basic skeleton for a go program with `cmd/` and `pkg/` folders, which
starts up, prints "hello world" and then stops.

## Step 2: Version

Implement the `--version` flag, using the cobra library. And arrange for the
version to be a build flag.

## Step 3: init subcommand

This subcommand creates a basic `.config/devroom/devroom.toml` file. Note: you
can set the folder to be checked and updated with the `--rootdir PATH` option.
Most `devroom commands` allow you to set this rather than default to the current
working directory.

The first step is to check that the selected folder is a git repo. If it is 
not then devroom will warn the user that it is not running in a git repo 
and ask for confirmation before creating this file (default 'N'). 

If it already exists, then it will not overwrite but say that one exists and
exit. 

```toml
# The runtime should be docker or podman.
runtime = "podman"

# This can be any base image in the docker hub registry.
base_image = "ubuntu:latest"

# At the time of writing only Claude is supported.
summary_model = "claude sonnet 4.5"

# Uncomment this to add an initial script for loading useful tools etc.
# jumpstart_script = "scripts/jumpstart.sh"
```

The `runtime` should be heuristically chosen by checking if `docker` or `podman`
commands exist. If both exist, simply pick docker and issue a little message
saying that is what you have done.


## Step 4: build subcommand

This subcommand forces a fresh build of the basic devroom container image. It will
read the `.config/devroom/devroom.toml` file and use that to select the base
image (e.g. ubuntu:24.04). And it will execute the jumpstart script to load
any standard tools and dependencies.

If the configuration file is missing, `devroom` will explain what is missing and
provide a hint that `devroom init` can be used to set up an empty configuration
file. If the configuration file is present but options are missing, then an
error message clearly explaining the missing option should be given. In both
cases `devroom` should quit.


## Step 5: new subcommand

Implement the `new` subcommand which will create a new devroom, using the 
current folder to implicitly supply the repo's forge, organisation and 
project. It should accept options `--name` and `--branch`.

- `-n,--name NAME` sets the name of the devroom that will be created. The
  intention is this is a nickname for the feature/fix and a suitable branch
  name.
- `-b,--branch` if present, creates and checks out a feature branch with the
  same name as the devroom.
- 




