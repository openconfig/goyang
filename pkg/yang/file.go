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

package yang

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// TODO(borman): encapsulate all of this someday so you can parse
// two completely independent yang files with different Paths.

var (
	// Path is the list of directories to look for .yang files in.
	Path []string
	// pathMap is used to prevent adding dups in Path.
	pathMap = map[string]bool{}

	// revisionDateSuffixRegex matches on the revision-date portion of a YANG
	// file's name.
	revisionDateSuffixRegex = regexp.MustCompile(`^@\d{4}-\d{2}-\d{2}\.yang$`)
)

// AddPath adds the directories specified in p, a colon separated list
// of directory names, to Path, if they are not already in Path. Using
// multiple arguments is also supported.
func AddPath(paths ...string) {
	for _, path := range paths {
		for _, p := range strings.Split(path, ":") {
			if !pathMap[p] {
				pathMap[p] = true
				Path = append(Path, p)
			}
		}
	}
}

// PathsWithModules returns all paths under and including the
// root containing files with a ".yang" extension, as well as
// any error encountered
func PathsWithModules(root string) (paths []string, err error) {
	pm := map[string]bool{}
	filepath.Walk(root, func(p string, info os.FileInfo, e error) error {
		err = e
		if err == nil {
			if info == nil {
				return nil
			}
			if !info.IsDir() && strings.HasSuffix(p, ".yang") {
				dir := filepath.Dir(p)
				if !pm[dir] {
					pm[dir] = true
					paths = append(paths, dir)
				}
			}
			return nil
		}
		return err
	})
	return
}

// readFile makes testing of findFile easier.
var readFile = ioutil.ReadFile

// scanDir makes testing of findFile easier.
var scanDir = findInDir

// findFile returns the name and contents of the .yang file associated with
// name, or an error.  If name is a module name rather than a file name (it does
// not have a .yang extension and there is no / in name), .yang is appended to
// the the name.  The directory that the .yang file is found in is added to Path
// if not already in Path. If a file is not found by exact match, directories
// are scanned for "name@revision-date.yang" files, the latest (sorted by
// YYYY-MM-DD revision-date) of these will be selected.
//
// If a path has the form dir/... then dir and all direct or indirect
// subdirectories of dir are searched.
//
// The current directory (.) is always checked first, no matter the value of
// Path.
func findFile(name string) (string, string, error) {
	slash := strings.Index(name, "/")
	if slash < 0 && !strings.HasSuffix(name, ".yang") {
		name += ".yang"
		if best := scanDir(".", name, false); best != "" {
			// we found a matching candidate in the local directory
			name = best
		}
	}

	switch data, err := readFile(name); true {
	case err == nil:
		AddPath(filepath.Dir(name))
		return name, string(data), nil
	case slash >= 0:
		// If there are any /'s in the name then don't search Path.
		return "", "", fmt.Errorf("no such file: %s", name)
	}

	for _, dir := range Path {
		var n string
		if filepath.Base(dir) == "..." {
			n = scanDir(filepath.Dir(dir), name, true)
		} else {
			n = scanDir(dir, name, false)
		}
		if n == "" {
			continue
		}
		if data, err := readFile(n); err == nil {
			return n, string(data), nil
		}
	}
	return "", "", fmt.Errorf("no such file: %s", name)
}

// findInDir looks for a file named name in dir or any of its subdirectories if
// recurse is true. if recurse is false, scan only the directory dir.
// If no matching file is found, an empty string is returned.
//
// The file SHOULD have the following name, per
// https://tools.ietf.org/html/rfc7950#section-5.2:
// module-or-submodule-name ['@' revision-date] '.yang'
// where revision-date = 4DIGIT "-" 2DIGIT "-" 2DIGIT
//
// If a perfect name match is found, then that file's path is returned.
// Else if file(s) with otherwise matching names but which contain a
// revision-date pattern exactly matching the above are found, then path of the
// one with the latest date is returned.
func findInDir(dir, name string, recurse bool) string {
	fis, err := ioutil.ReadDir(dir)
	if err != nil {
		return ""
	}

	var revisions []string
	mname := strings.TrimSuffix(name, ".yang")
	for _, fi := range fis {
		switch {
		case !fi.IsDir():
			if fn := fi.Name(); fn == name {
				return filepath.Join(dir, name)
			} else if strings.HasPrefix(fn, mname) && revisionDateSuffixRegex.MatchString(strings.TrimPrefix(fn, mname)) {
				revisions = append(revisions, fn)
			}
		case recurse:
			if n := findInDir(filepath.Join(dir, fi.Name()), name, recurse); n != "" {
				return n
			}
		}
	}
	if len(revisions) == 0 {
		return ""
	}
	sort.Strings(revisions)
	return filepath.Join(dir, revisions[len(revisions)-1])
}
