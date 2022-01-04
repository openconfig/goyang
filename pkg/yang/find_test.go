package yang

import (
	"testing"

	"github.com/openconfig/gnmi/errdiff"
)

func TestFindGrouping(t *testing.T) {
	tests := []struct {
		desc              string
		inMods            map[string]string
		inNode            func(*Modules) (Node, error)
		inName            string
		wantGroupNodePath string
		wantErrSubstr     string
	}{{
		desc: "grouping within module",
		inMods: map[string]string{
			"dev": `
				module dev {
					prefix d;
					namespace "urn:d";

					revision 01-01-01 { description "the start of time"; }

					grouping g { leaf a { type string; } }

					container c { leaf b { type string; } }
				}`,
		},
		inNode: func(ms *Modules) (Node, error) {
			return FindNode(ms.Modules["dev"], "c")
		},
		inName:            "g",
		wantGroupNodePath: "/dev/g",
	}, {
		desc: "nested grouping within module",
		inMods: map[string]string{
			"dev": `
				module dev {
					prefix d;
					namespace "urn:d";

					revision 01-01-01 { description "the start of time"; }

					grouping g { grouping gg { leaf a { type string; } } uses gg; }

					container c { leaf b { type string; } }
				}`,
		},
		inNode: func(ms *Modules) (Node, error) {
			return FindNode(ms.Modules["dev"], "g")
		},
		inName:            "gg",
		wantGroupNodePath: "/dev/g/gg",
	}, {
		desc: "grouping that uses another grouping both within the same module",
		inMods: map[string]string{
			"dev": `
				module dev {
					prefix d;
					namespace "urn:d";

					revision 01-01-01 { description "the start of time"; }

					grouping gg { leaf a { type string; } }

					grouping g { uses gg; }

					container c { leaf b { type string; } }
				}`,
		},
		inNode: func(ms *Modules) (Node, error) {
			return FindNode(ms.Modules["dev"], "g")
		},
		inName:            "gg",
		wantGroupNodePath: "/dev/gg",
	}, {
		desc: "grouping in included submodule",
		inMods: map[string]string{
			"dev": `
				module dev {
					prefix d;
					namespace "urn:d";
					include sys;

					container c { leaf b { type string; } }

					revision 01-01-01 { description "the start of time"; }
				}`,
			"sys": `
				submodule sys {
					belongs-to dev {
						prefix "d";
					}

					revision 01-01-01 { description "the start of time"; }

					grouping g { leaf a { type string; } }
				}`,
		},
		inNode: func(ms *Modules) (Node, error) {
			return FindNode(ms.Modules["dev"], "c")
		},
		inName:            "g",
		wantGroupNodePath: "/sys/g",
	}, {
		desc: "grouping in indirectly-included submodule",
		inMods: map[string]string{
			"dev": `
				module dev {
					prefix d;
					namespace "urn:d";
					include sys;

					revision 01-01-01 { description "the start of time"; }

					container c { leaf b { type string; } }
				}`,
			"sys": `
				submodule sys {
					belongs-to dev {
						prefix "d";
					}
					include sysdb;

					revision 01-01-01 { description "the start of time"; }
				}`,
			"sysdb": `
				submodule sysdb {
					belongs-to dev {
						prefix "d";
					}

					revision 01-01-01 { description "the start of time"; }

					grouping g { leaf a { type string; } }
				}`,
		},
		inNode: func(ms *Modules) (Node, error) {
			return FindNode(ms.Modules["dev"], "c")
		},
		inName:            "g",
		wantGroupNodePath: "/sysdb/g",
	}, {
		desc: "grouping in indirectly-included submodule with node in submodule",
		inMods: map[string]string{
			"dev": `
				module dev {
					prefix d;
					namespace "urn:d";
					include sys;

					revision 01-01-01 { description "the start of time"; }
				}`,
			"sys": `
				submodule sys {
					belongs-to dev {
						prefix "d";
					}
					include sysdb;

					revision 01-01-01 { description "the start of time"; }

					container c { leaf b { type string; } }
				}`,
			"sysdb": `
				submodule sysdb {
					belongs-to dev {
						prefix "d";
					}

					revision 01-01-01 { description "the start of time"; }

					grouping g { leaf a { type string; } }
				}`,
		},
		inNode: func(ms *Modules) (Node, error) {
			return FindNode(ms.SubModules["sys"], "c")
		},
		inName:            "g",
		wantGroupNodePath: "/sysdb/g",
	}, {
		desc: "grouping in submodule",
		inMods: map[string]string{
			"dev": `
				module dev {
					prefix d;
					namespace "urn:d";
					import sysdb { prefix "s"; }

					revision 01-01-01 { description "the start of time"; }

					container c { leaf b { type string; } uses s:g; }
				}`,
			"sysdb": `
				module sysdb {
					prefix sd;
					namespace "urn:sd";

					revision 01-01-01 { description "the start of time"; }

					grouping g { leaf a { type string; } }
				}`,
		},
		inNode: func(ms *Modules) (Node, error) {
			return FindNode(ms.Modules["dev"], "c")
		},
		inName:            "s:g",
		wantGroupNodePath: "/sysdb/g",
	}, {
		desc: "grouping that uses another grouping both in different modules",
		inMods: map[string]string{
			"dev": `
				module dev {
					prefix d;
					namespace "urn:d";
					import dev2 { prefix "de2"; }

					revision 01-01-01 { description "the start of time"; }

					container c { leaf l { type string; } uses de2:g; }
				}`,
			"dev2": `
				module dev2 {
					prefix d2;
					namespace "urn:d2";
					import dev3 { prefix "de3"; }

					revision 01-01-01 { description "the start of time"; }

					grouping g { leaf a { type string; } uses de3:gg; }
				}`,
			"dev3": `
				module dev3 {
					prefix d3;
					namespace "urn:d3";

					revision 01-01-01 { description "the start of time"; }

					grouping gg { leaf b { type string; } }
				}`,
		},
		inNode: func(ms *Modules) (Node, error) {
			return FindNode(ms.Modules["dev2"], "g")
		},
		inName:            "de3:gg",
		wantGroupNodePath: "/dev3/gg",
	}}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			ms := NewModules()

			for n, m := range tt.inMods {
				if err := ms.Parse(m, n); err != nil {
					t.Fatalf("cannot parse module %s, err: %v", n, err)
				}
			}

			errs := ms.Process()
			var err error
			switch len(errs) {
			case 1:
				err = errs[0]
				fallthrough
			case 0:
				if diff := errdiff.Substring(err, tt.wantErrSubstr); diff != "" {
					t.Fatalf("%s", diff)
				}
			default:
				t.Fatalf("got multiple errors: %v", errs)
			}

			seen := map[string]bool{}
			node, err := tt.inNode(ms)
			if err != nil {
				t.Fatalf("cannot find input node: %v", err)
			}
			g := FindGrouping(node, tt.inName, seen)
			if g == nil {
				t.Fatalf("cannot find grouping")
			}
			if got, want := NodePath(g), tt.wantGroupNodePath; got != want {
				t.Errorf("found grouping path doesn't match expected, got: %s, want: %s", got, want)
			}
		})
	}
}
