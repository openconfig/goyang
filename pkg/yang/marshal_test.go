// Copyright 2017 Google Inc.
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
	"encoding/json"
	"fmt"
	"testing"

	"github.com/kylelemons/godebug/pretty"
)

func TestMarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		in      *Entry
		want    string
		wantErr bool
	}{{
		name: "simple leaf entry",
		in: &Entry{
			Name: "leaf",
			Node: &Leaf{
				Name: "leaf",
			},
			Description: "This is a fake leaf.",
			Default:     "default-leaf-value",
			Errors:      []error{fmt.Errorf("error one")},
			Kind:        LeafEntry,
			Config:      TSTrue,
			Prefix: &Value{
				Name: "ModulePrefix",
				Source: &Statement{
					Keyword:     "prefix",
					Argument:    "ModulePrefix",
					HasArgument: true,
				},
			},
			Type: &YangType{
				Name:    "string",
				Kind:    Ystring,
				Default: "string-value",
			},
			Annotation: map[string]interface{}{
				"fish": struct{ Side string }{"chips"},
			},
		},
		want: `{
  "Name": "leaf",
  "Description": "This is a fake leaf.",
  "Default": "default-leaf-value",
  "Kind": 0,
  "Config": 1,
  "Prefix": {
    "Name": "ModulePrefix",
    "Source": {
      "Keyword": "prefix",
      "HasArgument": true,
      "Argument": "ModulePrefix"
    }
  },
  "Type": {
    "Name": "string",
    "Kind": 18,
    "Default": "string-value"
  },
  "Annotation": {
    "fish": {
      "Side": "chips"
    }
  }
}`,
	}, {
		name: "simple container entry with parent",
		in: &Entry{
			Name: "container",
			Node: &Container{
				Name: "container",
			},
			Kind:   DirectoryEntry,
			Config: TSFalse,
			Prefix: &Value{
				Name: "ModulePrefix",
				Source: &Statement{
					Keyword:     "prefix",
					Argument:    "ModulePrefix",
					HasArgument: true,
				},
			},
			Dir: map[string]*Entry{
				"child": {
					Name: "leaf",
					Node: &Leaf{
						Name: "leaf",
					},
					Kind:   LeafEntry,
					Config: TSUnset,
					Prefix: &Value{
						Name: "ModulePrefix",
						Source: &Statement{
							Keyword:     "prefix",
							Argument:    "ModulePrefix",
							HasArgument: true,
						},
					},
					Type: &YangType{
						Name: "union",
						Type: []*YangType{{
							Name:    "string",
							Pattern: []string{"^a.*$"},
							Kind:    Ystring,
							Length: YangRange{{
								Min: FromInt(10),
								Max: FromInt(20),
							}},
						}},
					},
				},
			},
		},
		want: `{
  "Name": "container",
  "Kind": 1,
  "Config": 2,
  "Prefix": {
    "Name": "ModulePrefix",
    "Source": {
      "Keyword": "prefix",
      "HasArgument": true,
      "Argument": "ModulePrefix"
    }
  },
  "Dir": {
    "child": {
      "Name": "leaf",
      "Kind": 0,
      "Config": 0,
      "Prefix": {
        "Name": "ModulePrefix",
        "Source": {
          "Keyword": "prefix",
          "HasArgument": true,
          "Argument": "ModulePrefix"
        }
      },
      "Type": {
        "Name": "union",
        "Kind": 0,
        "Type": [
          {
            "Name": "string",
            "Kind": 18,
            "Length": [
              {
                "Min": {
                  "Kind": 0,
                  "Value": 10,
                  "FractionDigits": 0
                },
                "Max": {
                  "Kind": 0,
                  "Value": 20,
                  "FractionDigits": 0
                }
              }
            ],
            "Pattern": [
              "^a.*$"
            ]
          }
        ]
      }
    }
  }
}`,
	}, {
		name: "Entry with list and leaflist",
		in: &Entry{
			Name:   "list",
			Kind:   DirectoryEntry,
			Config: TSUnset,
			Dir: map[string]*Entry{
				"leaf": {
					Name: "string",
					Kind: LeafEntry,
				},
				"leaf-list": {
					Name:     "leaf-list",
					ListAttr: &ListAttr{},
				},
			},
			ListAttr: &ListAttr{
				MaxElements: &Value{
					Name: "42",
				},
				MinElements: &Value{
					Name: "48",
				},
			},
			Identities: []*Identity{{
				Name: "ID_ONE",
			}},
			Exts: []*Statement{{
				Keyword:     "some-extension:ext",
				Argument:    "ext-value",
				HasArgument: true,
			}},
		},
		want: `{
  "Name": "list",
  "Kind": 1,
  "Config": 0,
  "Dir": {
    "leaf": {
      "Name": "string",
      "Kind": 0,
      "Config": 0
    },
    "leaf-list": {
      "Name": "leaf-list",
      "Kind": 0,
      "Config": 0,
      "ListAttr": {
        "MinElements": null,
        "MaxElements": null,
        "OrderedBy": null
      }
    }
  },
  "Exts": [
    {
      "Keyword": "some-extension:ext",
      "HasArgument": true,
      "Argument": "ext-value"
    }
  ],
  "ListAttr": {
    "MinElements": {
      "Name": "48"
    },
    "MaxElements": {
      "Name": "42"
    },
    "OrderedBy": null
  },
  "Identities": [
    {
      "Name": "ID_ONE"
    }
  ]
}`,
	}}

	for _, tt := range tests {
		got, err := json.MarshalIndent(tt.in, "", "  ")
		if err != nil {
			if !tt.wantErr {
				t.Errorf("%s: json.MarshalIndent(%v, ...): got unexpected error: %v", tt.name, tt.in, err)
			}
			continue
		}

		if diff := pretty.Compare(string(got), tt.want); diff != "" {
			t.Errorf("%s: jsonMarshalIndent(%v, ...): did not get expected JSON, diff(-got,+want):\n%s", tt.name, tt.in, diff)
		}
	}
}

func TestParseAndMarshal(t *testing.T) {
	tests := []struct {
		name string
		in   []inputModule
		want map[string]string
	}{{
		name: "simple single module",
		in: []inputModule{{
			name: "test.yang",
			content: `module test {
											prefix "t";
											namespace "urn:t";

											typedef foobar {
												type string {
													length "10";
												}
											}

											identity "BASE";
											identity "DERIVED" { base "BASE"; }

											container test {
												list a {
													key "k";
													leaf k { type string; }

													leaf bar {
														type foobar;
													}
												}

												leaf d {
													type decimal64 {
														fraction-digits 8;
													}
												}

												leaf-list zip {
													type string;
												}

												leaf x {
													type union {
														type string;
														type identityref {
															base "BASE";
														}
													}
												}
											}
										}`,
		}},
		want: map[string]string{
			"test": `{
  "Name": "test",
  "Kind": 1,
  "Config": 0,
  "Prefix": {
    "Name": "t",
    "Source": {
      "Keyword": "prefix",
      "HasArgument": true,
      "Argument": "t"
    }
  },
  "Dir": {
    "test": {
      "Name": "test",
      "Kind": 1,
      "Config": 0,
      "Prefix": {
        "Name": "t",
        "Source": {
          "Keyword": "prefix",
          "HasArgument": true,
          "Argument": "t"
        }
      },
      "Dir": {
        "a": {
          "Name": "a",
          "Kind": 1,
          "Config": 0,
          "Prefix": {
            "Name": "t",
            "Source": {
              "Keyword": "prefix",
              "HasArgument": true,
              "Argument": "t"
            }
          },
          "Dir": {
            "bar": {
              "Name": "bar",
              "Kind": 0,
              "Config": 0,
              "Prefix": {
                "Name": "t",
                "Source": {
                  "Keyword": "prefix",
                  "HasArgument": true,
                  "Argument": "t"
                }
              },
              "Type": {
                "Name": "foobar",
                "Kind": 18,
                "Length": [
                  {
                    "Min": {
                      "Kind": 0,
                      "Value": 10,
                      "FractionDigits": 0
                    },
                    "Max": {
                      "Kind": 0,
                      "Value": 10,
                      "FractionDigits": 0
                    }
                  }
                ]
              }
            },
            "k": {
              "Name": "k",
              "Kind": 0,
              "Config": 0,
              "Prefix": {
                "Name": "t",
                "Source": {
                  "Keyword": "prefix",
                  "HasArgument": true,
                  "Argument": "t"
                }
              },
              "Type": {
                "Name": "string",
                "Kind": 18
              }
            }
          },
          "Key": "k",
          "ListAttr": {
            "MinElements": null,
            "MaxElements": null,
            "OrderedBy": null
          }
        },
        "d": {
          "Name": "d",
          "Kind": 0,
          "Config": 0,
          "Prefix": {
            "Name": "t",
            "Source": {
              "Keyword": "prefix",
              "HasArgument": true,
              "Argument": "t"
            }
          },
          "Type": {
            "Name": "decimal64",
            "Kind": 12,
            "FractionDigits": 8,
            "Range": [
              {
                "Min": {
                  "Kind": 2,
                  "Value": 0,
                  "FractionDigits": 0
                },
                "Max": {
                  "Kind": 3,
                  "Value": 0,
                  "FractionDigits": 0
                }
              }
            ]
          }
        },
        "x": {
          "Name": "x",
          "Kind": 0,
          "Config": 0,
          "Prefix": {
            "Name": "t",
            "Source": {
              "Keyword": "prefix",
              "HasArgument": true,
              "Argument": "t"
            }
          },
          "Type": {
            "Name": "union",
            "Kind": 19,
            "Type": [
              {
                "Name": "string",
                "Kind": 18
              },
              {
                "Name": "identityref",
                "Kind": 15,
                "IdentityBase": {
                  "Name": "BASE",
                  "Values": [
                    {
                      "Name": "DERIVED"
                    }
                  ]
                }
              }
            ]
          }
        },
        "zip": {
          "Name": "zip",
          "Kind": 0,
          "Config": 0,
          "Prefix": {
            "Name": "t",
            "Source": {
              "Keyword": "prefix",
              "HasArgument": true,
              "Argument": "t"
            }
          },
          "Type": {
            "Name": "string",
            "Kind": 18
          },
          "ListAttr": {
            "MinElements": null,
            "MaxElements": null,
            "OrderedBy": null
          }
        }
      }
    }
  },
  "Identities": [
    {
      "Name": "BASE",
      "Values": [
        {
          "Name": "DERIVED"
        }
      ]
    },
    {
      "Name": "DERIVED"
    }
  ]
}`,
		},
	}, {
		name: "multiple modules with extension",
		in: []inputModule{{
			name: "ext.yang",
			content: `module ext {
										prefix "e";
										namespace "urn:e";

										extension foobar {
											argument "baz";
										}
									}`,
		}, {
			name: "test.yang",
			content: `module test {
											prefix "t";
											namespace "urn:t";

											import ext { prefix ext; }

											leaf t {
												type string;
												ext:foobar "marked";
											}
										}`,
		}},
		want: map[string]string{
			"test": `{
  "Name": "test",
  "Kind": 1,
  "Config": 0,
  "Prefix": {
    "Name": "t",
    "Source": {
      "Keyword": "prefix",
      "HasArgument": true,
      "Argument": "t"
    }
  },
  "Dir": {
    "t": {
      "Name": "t",
      "Kind": 0,
      "Config": 0,
      "Prefix": {
        "Name": "t",
        "Source": {
          "Keyword": "prefix",
          "HasArgument": true,
          "Argument": "t"
        }
      },
      "Type": {
        "Name": "string",
        "Kind": 18
      },
      "Exts": [
        {
          "Keyword": "ext:foobar",
          "HasArgument": true,
          "Argument": "marked"
        }
      ]
    }
  }
}`,
			"ext": `{
  "Name": "ext",
  "Kind": 1,
  "Config": 0,
  "Prefix": {
    "Name": "e",
    "Source": {
      "Keyword": "prefix",
      "HasArgument": true,
      "Argument": "e"
    }
  }
}`,
		},
	}}

	for _, tt := range tests {
		ms := NewModules()

		for _, mod := range tt.in {
			if err := ms.Parse(mod.content, mod.name); err != nil {
				t.Errorf("%s: ms.Parse(..., %v): parsing error with module: %v", tt.name, mod.name, err)
				continue
			}

			if errs := ms.Process(); len(errs) != 0 {
				t.Errorf("%s: ms.Process(): could not parse modules: %v", tt.name, errs)
				continue
			}

			entries := make(map[string]*Entry)
			for _, m := range ms.Modules {
				if _, ok := entries[m.Name]; !ok {
					entries[m.Name] = ToEntry(m)

					got, err := json.MarshalIndent(entries[m.Name], "", "  ")
					if err != nil {
						t.Errorf("%s: json.MarshalIndent(...): got unexpected error: %v", tt.name, err)
						continue
					}

					if diff := pretty.Compare(string(got), tt.want[m.Name]); diff != "" {
						t.Errorf("%s: json.MarshalIndent(...): did not get expected JSON, diff(-got,+want):\n%s", tt.name, diff)
					}
				}
			}
		}
	}
}
