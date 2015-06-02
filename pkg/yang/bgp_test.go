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

import "testing"

// TestBGP simply makes sure we are able to parse a version of Anees's
// BGP model.  We don't actually attempt to validate we got the right
// AST.  ast_test.go will test smaller peices to make sure the basics
// of BuildAST produce expected results.
func TestBGP(t *testing.T) {
	ss, err := Parse(bgp, "bgp.yang")
	if err != nil {
		t.Fatal(err)
	}
	if len(ss) != 1 {
		t.Fatalf("got %d results, want 1", len(ss))
	}
	_, err = BuildAST(ss[0])
	if err != nil {
		t.Fatal(err)
	}
}

var bgp = `
module google-bgp {

  yang-version "1";

  // namespace
  namespace "http://google.com/yang/google-bgp-protocol-cfg";

  prefix "gbgp";

  // import some basic types -- no other dependency on
  // in-progress models in draft status
  import ietf-inet-types { prefix inet; }


  // meta
  organization "Google, Inc.";

  contact
    "Google, Inc.
    1600 Amphitheatre Way
    Mountain View, CA  94043";

  description
    "This module describes a YANG model for BGP protocol
    configuration.It is a limited subset of all of the configuration
    parameters available in the variety of vendor implementations,
    hence it is expected that it would be augmented with vendor-
    specific configuration data as needed.Additional modules or
    submodules to handle other aspects of BGP configuration,
    including policy, VRFs, and additional address families are also 
    expected.";

  revision "2014-07-07" {
    description
      "Initial revision";
    reference "TBD";
  }


  identity afi-type {
    description
      "base identity type for BGP address family identifiers (AFI)";
    reference "IETF RFC 4760";
  }

  identity safi-type {
    description
      "base identity type for BGP subsequent address family
      identifiers (SAFI)";
    reference "IETF RFC 4760";
  }

  identity ipv4-afi {
    base afi-type;
    description
      "IPv4 AF identifier";
  }

  identity ipv6-afi {
    base afi-type;
    description
      "IPv6 AF identifier";
  }

  identity unicast-safi {
    base safi-type;
    description
      "unicast SAFI identifier";
  }

  identity labeled-unicast-safi {
    base safi-type;
    description
      "labeled unicast SAFI identifier";
    reference "RFC 3107 - Carrying Label Information in BGP-4";
  }


  typedef peer-group-type {
    type enumeration {
      enum INTERNAL {
        description "internal (iBGP) peer";
      }
      enum EXTERNAL {
        description "external (eBGP) peer";
      }
    }
    description
      "labels a peer as explicitly internal or external";
  }


  typedef remove-private-as-option {
    type enumeration {
      enum ALL {
        description "remove all private ASes in the path";
      }
      enum REPLACE {
        description "replace private ASes with local AS";
      }
    }
    description
      "set of options for configuring how private AS path numbers
      are removed from advertisements";
  }

  typedef percentage {
    type uint8 {
      range "0..100";
    }
    description
      "Integer indicating a percentage value";
  }

  typedef rr-cluster-id-type {
    type union {
      type uint32;
      type inet:ipv4-address;
    }
    description
      "union type for route reflector cluster ids:
      option 1: 4-byte number
      option 2: IP address";
  }

  grouping bgp-common-configuration {
    description "Common configuration across neighbors, groups,
    etc.";

    leaf description {
      type string;
      description
        "A textual description of the peer or group";
    }
    container use-multiple-paths {
      description
        "Configuration of BGP multipath to enable load sharing across
        multiple paths to peers.";
      leaf allow-multiple-as {
        type boolean;
        default "false";
        description
          "Allow multipath to use paths from different neighboring
          ASes.  The default is to only consider multiple paths from
          the same neighboring AS.";
      }
      leaf maximum-paths {
        type uint32;
        default 1;
        description
          "Maximum number of parallel paths to consider when using
          BGP multipath.  The default is to use a single path.";
        reference "draft-ietf-idr-add-paths-09.txt";
      }
    }

  }

  grouping bgp-group-common-configuration {
    description "Configuration items that are applied at the peer
    group level";
  }

  grouping bgp-group-neighbor-common-configuration {
    description "Configuration options for peer and group context";

    leaf auth-password {
      type string;
      description
        "Configures an authentication password for use with
        neighboring devices.";
    }

    container timers {
      description "Configuration of various BGP timers";

      leaf hold-time {
        type decimal64 {
          fraction-digits 2;
        }
        default 90;
        // hold-time should typically be set to 3x the
        // keepalive-interval -- create a constraint for this?
        description
          "Time interval in seconds that a BGP session will be
          considered active in the absence of keepalive or other
          messages from the peer";
        reference
          "RFC 1771 - A Border Gateway Protocol 4";
      }

      leaf keepalive-interval {
        type decimal64 {
          fraction-digits 2;
        }
        default 30;
        description
          "Time interval in seconds between transmission of keepalive
          messages to the neighbor.  Typically set to 1/3 the 
          hold-time.";
      }

      leaf advertisement-interval {
        type decimal64 {
          fraction-digits 2;
        }
        default 30;
        description
          "Mininum time interval in seconds between transmission 
          of BGP updates to neighbors";
        reference
          "RFC 1771 - A Border Gateway Protocol 4";
      }

      leaf connect-retry {
        type decimal64 {
          fraction-digits 2;
        }
        default 30;
        description
          "Time interval in seconds between attempts to establish a
          session with the peer.";
      }
    }

    container ebgp-multihop {
      description
        "Configure multihop BGP for peers that are not directly
        connected";

      leaf multihop-ttl {
        type uint8;
        default 1;
        description
          "Time-to-live for multihop BGP sessions.  The default
          value of 1 is for directly connected peers (i.e.,
          multihop disabled";

      }

    }

    container route-reflector {
      description
        "Configure the local router as a route-reflector
        server";
      leaf route-reflector-clusterid {
        type rr-cluster-id-type;
        description
          "route-reflector cluster id to use when local router is
          configured as a route reflector.  Commonly set at the group
          level, but allows a different cluster
          id to be set for each neighbor.";
      }

      leaf route-reflector-client {
        type boolean;
        default "false";
        description
          "configure the neighbor as a route reflector client";
      }
    }

    leaf remove-private-as {
      // could also make this a container with a flag to enable
      // remove-private and separate option.  here, option implies 
      // remove-private is enabled.
      type remove-private-as-option;
      description
        "Remove private AS numbers from updates sent to peers";
    }


    container bgp-logging-options {
      description
        "Configure various tracing/logging options for BGP peers
        or groups.  Expected that additional vendor-specific log
        options would augment this container";

      leaf log-neighbor-state-changes {
        type boolean;
        default "true";
        description
          "Configure logging of peer state changes.  Default is
          to enable logging of peer state changes.";
      }
    }

    container transport-options {
      description
        "Transport protocol options for BGP sessions";

        leaf tcp-mss {
          type uint16;
          description
            "Sets the max segment size for BGP TCP sessions";
        }

        leaf passive-mode {
          type boolean;
          description
            "Wait for peers to issue requests to open a BGP session,
            rather than initiating sessions from the local router";
        }
    }

    leaf local-address {
      type inet:ip-address;
      description
        "Set the local IP (either IPv4 or IPv6) address to use for
        the session when sending BGP update messages";
    }

    leaf route-flap-damping {
      type boolean;
      description
        "Enable route flap damping";
    }
  }

  grouping bgp-address-family-common-configuration {
    description "Configuration options per address family context";

    list address-family {

      key "afi-name";
      description
        "Per address-family configuration, uniquely identified by AF
        name"; 
      leaf afi-name {
        type identityref {
          base "afi-type";
        }
        description
          "Address family names are drawn from the afi-type base
          identity, which has specific address family types as
          derived identities";
      }

      list subsequent-address-family {

        key "safi-name";
        description
          "Per subsequent address family configuration, under a
          specific address family";

        leaf safi-name {
          // do we need to specify which SAFIs are possible within
          // each AF? with the current set  of AF/SAFI, all are
          /// applicable
          type identityref {
            base "safi-type";
          }
          description
            "Within each address family, subsequent address family
            names are drawn from the subsequent-address-family base
            identity";
        } 
   

        container prefix-limit {
          description
          "Configure the maximum number of prefixes that will be
          accepted from a peer";

          leaf max-prefixes {
            type uint32;
            description
              "Maximum number of prefixes that will be accepted from
              the neighbor";
          }

          leaf shutdown-threshold-pct {
            type percentage;
            description
              "Threshold on number of prefixes that can be received
              from a neighbor before generation of warning messages
              or log entries.  Expressed as a percentage of
              max-prefixes.";
          }

          leaf restart-timer {
            type decimal64 {
              fraction-digits 2;
            }
            units "seconds";
            description
              "Time interval in seconds after which the BGP session
              is reestablished after being torn down due to exceeding
              the max-prefixes limit.";
          }
        }
       }
    }
  }



  container bgp {
    description "Top-level configuration data for the BGP router";

    container global {
      description
        "Top-level bgp protocol options applied across peer-groups,
        neighbors, and address families";

      leaf as {
        type inet:as-number;
        mandatory "true";
        description
          "Local autonomous system number of the router.  Uses 
          the as-number type defined in RFC 6991";
      }
      leaf router-id {
        type inet:ipv4-address;
        description
          "Router id of the router, expressed as an
          IPv4 address";
          // there is a typedef for this in draft module ietf-routing
          // but it does not use an appropriate type
      }
      container route-selection-options {
        description
          "Set of configuration options that govern best
          path selection";
        leaf always-compare-med {
          type boolean;
          default "false";
          description
            "Compare multi-exit discriminator (MED) value from 
            different ASes when selecting the best route.  The
            default behavior is to only compare MEDs for paths
            received from the same AS.";
        }
        leaf ignore-as-path {
          type boolean;
          default "false";
          description
            "Ignore the AS path length when selecting the best path.
            The default is to use the AS path length and prefer paths
            with shorter length.";
        }
        leaf external-compare-router-id {
          type boolean;
          default "true";
          description
            "When comparing similar routes received from external
            BGP peers, use the router-id as a criterion to select 
            the active path.  The default is to use the router-id to
            select among similar routes.";
        }
        leaf advertise-inactive-routes {
          type boolean;
          default "false";
          description
            "Advertise inactive routes to external peers.  The
            default is to only advertise active routes.";
        }
      }
      container default-route-distance {
        description
          "Administrative distance (or preference) assigned to
          routes received from different sources
          (external, internal, and local.)";
        leaf external-route-distance {
          type uint8 {
            range "1..255";
          }
          description
            "Administrative distance for routes learned from external
            BGP (eBGP)";
        }
        leaf internal-route-distance {
          type uint8 {
            range "1..255";
          }
          description
            "Administrative distance for routes learned from internal
            BGP (iBGP)";

        }
      }
    }

    uses bgp-address-family-common-configuration;

    list peer-group {
      key "group-name";
      description
        "List of peer-groups, uniquely identified by the peer group
        names";
      leaf group-name {
        type string;
        description "Name of the peer group";
      }
      leaf group-type {
        type peer-group-type;
        description
          "Explicitly designate the peer group as internal (iBGP)
          or external (eBGP)";
      }
      uses bgp-common-configuration;
      uses bgp-address-family-common-configuration;
      uses bgp-group-neighbor-common-configuration;
    }

    list neighbor {
      key "neighbor-address";
      description
        "List of BGP peers, uniquely identified by neighbor address";
      leaf neighbor-address {
        type inet:ip-address;
        description
          "Address of the BGP peer, either IPv4 or IPv6";
      }

      leaf peer-as {
        type inet:as-number;
        mandatory "true";
        description
          "AS number of the peer";

      }
      uses bgp-common-configuration;
      uses bgp-address-family-common-configuration;
      uses bgp-group-neighbor-common-configuration;
    }

  }
}`
