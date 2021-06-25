![Go](https://github.com/openconfig/goyang/workflows/Go/badge.svg?branch=master)
[![Coverage Status](https://coveralls.io/repos/github/openconfig/goyang/badge.svg?branch=master)](https://coveralls.io/github/openconfig/goyang?branch=master)

Current support for `goyang` is for the [latest 3 Go releases](https://golang.org/project/#release).

# goyang
YANG parser and compiler for Go programs.

The yang package (pkg/yang) is used to convert a YANG schema into either an
in memory abstract syntax trees (ast) or more fully resolved, in memory, "Entry"
trees.  An Entry tree consists only of Entry structures and has had
augmentation, imports, and includes all applied.

goyang is a sample program that uses the yang (pkg/yang) package.

goyang uses the yang package to create an in-memory tree representation of
schemas defined in YANG and then dumps out the contents in several forms.
The forms include:

*  tree - a simple tree representation
*  types - list understood types extracted from the schema

The yang package, and the goyang program, are not complete and are a work in
progress.

For more complex output types, such as Go structs, and protobuf messages
please use the [openconfig/ygot](https://github.com/openconfig/ygot) package,
which uses this package as its backend.

### Getting started

To build goyang, ensure you have go language tools installed
(available at [golang.org](https://golang.org/dl)) and that the `GOPATH`
environment variable is set to your Go workspace.

1. `go get github.com/openconfig/goyang`
    * This will download goyang code and dependencies into the src
subdirectory in your workspace.

2. `cd <workspace>/src/github.com/openconfig/goyang`

3. `go build`

   * This will build the goyang binary and place it in the bin
subdirectory in your workspace.

### Contributing to goyang

goyang is still a work-in-progress and we welcome contributions.  Please see
the `CONTRIBUTING` file for information about how to contribute to the codebase.

### Disclaimer

This is not an official Google product.
