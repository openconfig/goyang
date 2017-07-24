// Copyright 2015 Google Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Program yang parses YANG files, displays errors, and possibly writes
// something related to the input on output.
//
// Usage: yang [--path DIR] [--format FORMAT] [FORMAT OPTIONS] [MODULE] [FILE ...]
//
// If MODULE is specified (an argument that does not end in .yang), it is taken
// as the name of the module to display.  Any FILEs specified are read, and the
// tree for MODULE is displayed.  If MODULE was not defined in FILEs (or no
// files were specified), then the file MODULES.yang is read as well.  An error
// is displayed if no definition for MODULE was found.
//
// If MODULE is missing, then all base modules read from the FILEs are
// displayed.  If there are no arguments then standard input is parsed.
//
// If DIR is specified, it is considered a comma separated list of paths
// to append to the search directory.  If DIR appears as DIR/... then
// DIR and all direct and indirect subdirectories are checked.
//
// FORMAT, which defaults to "tree", specifes the format of output to produce.
// Use "goyang --help" for a list of available formats.
//
// FORMAT OPTIONS are flags that apply to a specific format.  They must follow
// --format.
//
// THIS PROGRAM IS STILL JUST A DEVELOPMENT TOOL.
package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"runtime/trace"
	"sort"
	"strings"

	"github.com/openconfig/goyang/pkg/indent"
	"github.com/openconfig/goyang/pkg/yang"
	"github.com/pborman/getopt"
)

// Each format must register a formatter with register.  The function f will
// be called once with the set of yang Entry trees generated.
type formatter struct {
	name  string
	f     func(io.Writer, []*yang.Entry)
	help  string
	flags *getopt.Set
}

var formatters = map[string]*formatter{}

func register(f *formatter) {
	formatters[f.name] = f
}

// exitIfError writes errs to standard error and exits with an exit status of 1.
// If errs is empty then exitIfError does nothing and simply returns.
func exitIfError(errs []error) {
	if len(errs) > 0 {
		for _, err := range errs {
			fmt.Fprintln(os.Stderr, err)
		}
		stop(1)
	}
}

var stop = os.Exit

func main() {
	var format string
	formats := make([]string, 0, len(formatters))
	for k := range formatters {
		formats = append(formats, k)
	}
	sort.Strings(formats)

	var traceP string
	var help bool
	var paths []string
	getopt.ListVarLong(&paths, "path", 0, "comma separated list of directories to add to search path", "DIR[,DIR...]")
	getopt.StringVarLong(&format, "format", 0, "format to display: "+strings.Join(formats, ", "), "FORMAT")
	getopt.StringVarLong(&traceP, "trace", 0, "write trace into to TRACEFILE", "TRACEFILE")
	getopt.BoolVarLong(&help, "help", '?', "display help")
	getopt.BoolVarLong(&yang.ParseOptions.IgnoreSubmoduleCircularDependencies, "ignore-circdep", 0, "ignore circular dependencies between submodules")
	getopt.SetParameters("[FORMAT OPTIONS] [SOURCE] [...]")

	if err := getopt.Getopt(func(o getopt.Option) bool {
		if o.Name() == "--format" {
			f, ok := formatters[format]
			if !ok {
				fmt.Fprintf(os.Stderr, "%s: invalid format.  Choices are %s\n", format, strings.Join(formats, ", "))
				stop(1)
			}
			if f.flags != nil {
				f.flags.VisitAll(func(o getopt.Option) {
					getopt.AddOption(o)
				})
			}
		}
		return true
	}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		getopt.PrintUsage(os.Stderr)
		os.Exit(1)
	}

	if traceP != "" {
		fp, err := os.Create(traceP)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		trace.Start(fp)
		stop = func(c int) { trace.Stop(); os.Exit(c) }
		defer func() { trace.Stop() }()
	}

	if help {
		getopt.CommandLine.PrintUsage(os.Stderr)
		fmt.Fprintf(os.Stderr, `
SOURCE may be a module name or a .yang file.

Formats:
`)
		for _, fn := range formats {
			f := formatters[fn]
			fmt.Fprintf(os.Stderr, "    %s - %s\n", f.name, f.help)
			if f.flags != nil {
				f.flags.PrintOptions(indent.NewWriter(os.Stderr, "   "))
			}
			fmt.Fprintln(os.Stderr)
		}
		stop(0)
	}

	for _, path := range paths {
		expanded, err := yang.PathsWithModules(path)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			continue
		}
		yang.AddPath(expanded...)
	}

	if format == "" {
		format = "tree"
	}
	if _, ok := formatters[format]; !ok {
		fmt.Fprintf(os.Stderr, "%s: invalid format.  Choices are %s\n", format, strings.Join(formats, ", "))
		stop(1)

	}

	files := getopt.Args()

	ms := yang.NewModules()

	if len(files) == 0 {
		data, err := ioutil.ReadAll(os.Stdin)
		if err == nil {
			err = ms.Parse(string(data), "<STDIN>")
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			stop(1)
		}
	}

	for _, name := range files {
		if err := ms.Read(name); err != nil {
			fmt.Fprintln(os.Stderr, err)
			continue
		}
	}

	// Process the read files, exiting if any errors were found.
	exitIfError(ms.Process())

	// Keep track of the top level modules we read in.
	// Those are the only modules we want to print below.
	mods := map[string]*yang.Module{}
	var names []string

	for _, m := range ms.Modules {
		if mods[m.Name] == nil {
			mods[m.Name] = m
			names = append(names, m.Name)
		}
	}
	sort.Strings(names)
	entries := make([]*yang.Entry, len(names))
	for x, n := range names {
		entries[x] = yang.ToEntry(mods[n])
	}

	formatters[format].f(os.Stdout, entries)
}
