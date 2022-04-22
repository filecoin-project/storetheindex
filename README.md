# StoreTheIndex 🗂️
[![](https://img.shields.io/badge/made%20by-Protocol%20Labs-blue.svg?style=flat-square)](https://protocol.ai)
[![Go Reference](https://pkg.go.dev/badge/github.com/filecoin-project/storetheindex.svg)](https://pkg.go.dev/github.com/filecoin-project/storetheindex)
[![Coverage Status](https://codecov.io/gh/filecoin-project/storetheindex/branch/main/graph/badge.svg)](https://codecov.io/gh/filecoin-project/storetheindex/branch/main)
> The first place to go in order to find a CID stored in Filecoin

This repo provides an indexer implementation that can be used to index data stored by a range of participating storage providers.

## Design
- [About the Indexer](https://github.com/filecoin-project/storetheindex/blob/main/doc/indexer_about.md#about-the-indexer)
- [Ingestion Process](https://github.com/filecoin-project/storetheindex/blob/main/doc/ingest.md#providing-data-to-a-network-indexer)
- [Creating an Index Provicer](https://github.com/filecoin-project/storetheindex/blob/main/doc/creating-an-index-provider.md#creating-an-index-provider)

## Current Status
Released for production: The current production release is running at https://cid.contact 

This project and is currently under active development 🚧.  

## Install
This assumes go is already installed.

Install storetheindex:
```sh
go get github.com/filecoin-project/storetheindex
```

Initialize the storetheindex repository and configuration:
```sh
storetheindex init
```

## Running the Indexer Service
To run storetheindex as a service, run the `daemon` command. The service watches for providers to index, and exposes a query / content routing client interface.

The daemon is configured by the config file in the storetheindex repository. The config file and repo are created when storetheindex is initialized, using the `init` command. This repo is located in the local file system. By default, the repo is located at ~/.storetheindex. To change the repo location, set the `$STORETHEINDEX_PATH` environmental variable.

## Indexer CLI Commands
There are a number of client commands included with storetheindex. Their purpose is to perform simple indexing and lookup actions against a running daemon.  These can be helpful to test that an indexer is working. These include the following commands:

Informational:

- `find` Find value by CID or multihash in indexer
- `providers` Show information about providers known to the indexer
  - `get` Get information about a specified provider
  - `list` List the known providers

Administrative:

- `admin` Perform admin activities with an indexer
  - `allow` Allow advertisements and content from peer
  - `block` Block advertisements and content from peer
  - `reload` Reload the policy from the configuration file
  - `sync` Sync indexer with provider
- `init` Initialize or upgrade indexer node config file

Testing:

- `import` Imports data to indexer from different sources
- `register` Register provider information with an idexer
- `synthetic` Generate synthetic load to import in indexer

## Help
To see a list of available commands, see `storetheindex --help`. For help with command usage, see `storetheindex <command> --help`.


## Configuration
The storetheindex config file is a JSON document located at `$STORETHEINDEX_PATH`/config. It is read once, either for an offline command, or when starting the daemon. For documentation of the items in the config file, see the [godoc documentation](https://pkg.go.dev/github.com/filecoin-project/storetheindex/config) of the corresponding config data structures.

## License
[SPDX-License-Identifier: Apache-2.0 OR MIT](LICENSE.md)
