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
	"path"
	"path/filepath"
	"strings"
)

// TODO(borman): encapsulate all of this someday so you can parse
// two completely independent yang files with different Paths.

// Path is the list of directories to look for .yang files in.
var Path []string
var pathMap = map[string]bool{} // prevent adding dups in Path

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

// PathsWithModules returns all paths  under and including the
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
				dir := path.Dir(p)
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

// findFile returns the name and contents of the .yang file associated with
// name, or an error.  If name is a module name rather than a file name (it does
// not have a .yang extension and there is no / in name), .yang is appended to
// the the name.  The directory that the .yang file is found in is added to Path
// if not already in Path.
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
	}

	switch data, err := readFile(name); true {
	case err == nil:
		AddPath(path.Dir(name))
		return name, string(data), nil
	case slash >= 0:
		// If there are any /'s in the name then don't search Path.
		return "", "", fmt.Errorf("no such file: %s", name)
	}

	for _, dir := range Path {
		var n string
		if path.Base(dir) == "..." {
			n = findInDir(path.Dir(dir), name)
		} else {
			n = path.Join(dir, name)
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

// findInDir looks for a file named name in dir or any of its subdirectories.
func findInDir(dir, name string) string {
	fis, err := ioutil.ReadDir(dir)
	if err != nil {
		return ""
	}
	for _, fi := range fis {
		if !fi.IsDir() {
			if fi.Name() == name {
				return path.Join(dir, name)
			}
			continue
		}
		if n := findInDir(path.Join(dir, fi.Name()), name); n != "" {
			return n
		}
	}
	return ""
}
