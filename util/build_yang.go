package util

import (
	"fmt"

	"github.com/openconfig/goyang/pkg/yang"
)

// ProcessModules takes a list of modules, and a path specification and
// runs the yang parser against them, returning a slice of yang.Entry
// pointers which represent the top level modules that are to be parsed
// by the struct generation.
func ProcessModules(yangf, path []string) (map[string]*yang.Entry, []error) {
	for _, p := range path {
		yang.AddPath(fmt.Sprintf("%s/...", p))
	}

	ms := yang.NewModules()

	var processErr []error
	for _, name := range yangf {
		if err := ms.Read(name); err != nil {
			processErr = append(processErr, err)
		}
	}

	if len(processErr) > 0 {
		return nil, processErr
	}

	if errs := ms.Process(); len(errs) != 0 {
		return nil, errs
	}

	entries := make(map[string]*yang.Entry)
	for _, m := range ms.Modules {
		e := yang.ToEntry(m)
		entries[e.Name] = e
	}

	return entries, nil
}
