# ----------------------------------------------------------------------------
# "THE BEER-WARE LICENSE" (Revision 42):
# <xythobuz@xythobuz.de> wrote this file.  As long as you retain this notice
# you can do whatever you want with this stuff. If we meet some day, and you
# think this stuff is worth it, you can buy me a beer in return.   Thomas Buck
# ----------------------------------------------------------------------------

HOST = iot

# ----------------------------------------------------------------------------

# https://tech.davis-hansson.com/p/make/

SHELL := bash
.ONESHELL:
.SHELLFLAGS := -eu -o pipefail -c
.DELETE_ON_ERROR:
MAKEFLAGS += --warn-undefined-variables
MAKEFLAGS += --no-builtin-rules

# check for recent make
ifeq ($(origin .RECIPEPREFIX), undefined)
  $(error This Make does not support .RECIPEPREFIX. Please use GNU Make 4.0 or later)
endif
.RECIPEPREFIX = >

# ----------------------------------------------------------------------------

all: lights-telegram

lights-telegram: lights-telegram.go
> CGO_ENABLED=0 go build

clean:
> rm -rf lights-telegram

upload: lights-telegram
> ssh $(HOST) sudo systemctl stop lights-telegram
> scp lights-telegram $(HOST):~/bin/lights-telegram/lights-telegram
> sleep 1
> ssh $(HOST) sudo systemctl start lights-telegram
.PHONY: upload

download:
> scp $(HOST):~/bin/lights-telegram/config.yaml config.yaml
.PHONY: download
