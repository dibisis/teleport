/*
Copyright 2015 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package services

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
)

// Presence records and reports the presence of all components
// of the cluster - Nodes, Proxies and SSH nodes
type Presence interface {
	// GetNodes returns a list of registered servers
	GetNodes() ([]Server, error)

	// UpsertNode registers node presence, permanently if ttl is 0 or
	// for the specified duration with second resolution if it's >= 1 second
	UpsertNode(server Server, ttl time.Duration) error

	// GetAuthServers returns a list of registered servers
	GetAuthServers() ([]Server, error)

	// UpsertAuthServer registers auth server presence, permanently if ttl is 0 or
	// for the specified duration with second resolution if it's >= 1 second
	UpsertAuthServer(server Server, ttl time.Duration) error

	// UpsertProxy registers proxy server presence, permanently if ttl is 0 or
	// for the specified duration with second resolution if it's >= 1 second
	UpsertProxy(server Server, ttl time.Duration) error

	// GetProxies returns a list of registered proxies
	GetProxies() ([]Server, error)

	// UpsertReverseTunnel upserts reverse tunnel entry temporarily or permanently
	UpsertReverseTunnel(tunnel ReverseTunnel, ttl time.Duration) error

	// GetReverseTunnels returns a list of registered servers
	GetReverseTunnels() ([]ReverseTunnel, error)

	// DeleteReverseTunnel deletes reverse tunnel by it's domain name
	DeleteReverseTunnel(domainName string) error
}

// Site represents a cluster of teleport nodes who collectively trust the same
// certificate authority (CA) and have a common name.
//
// The CA is represented by an auth server (or multiple auth servers, if running
// in HA mode)
type Site struct {
	Name          string    `json:"name"`
	LastConnected time.Time `json:"lastconnected"`
	Status        string    `json:"status"`
}

// Server represents a node in a Teleport cluster
type Server struct {
	ID        string                  `json:"id"`
	Addr      string                  `json:"addr"`
	Hostname  string                  `json:"hostname"`
	Namespace string                  `json:"namespace"`
	Labels    map[string]string       `json:"labels"`
	CmdLabels map[string]CommandLabel `json:"cmd_labels"`
}

func (s *Server) GetNamespace() string {
	return ProcessNamespace(s.Namespace)
}

// ReverseTunnel is SSH reverse tunnel established between a local Proxy
// and a remote Proxy. It helps to bypass firewall restrictions, so local
// clusters don't need to have the cluster involved
type ReverseTunnel struct {
	// DomainName is a domain name of remote cluster we are connecting to
	DomainName string `json:"domain_name"`
	// DialAddrs is a list of remote address to establish a connection to
	// it's always SSH over TCP
	DialAddrs []string `json:"dial_addrs"`
}

// Check returns nil if all parameters are good, error otherwise
func (r *ReverseTunnel) Check() error {
	if strings.TrimSpace(r.DomainName) == "" {
		return trace.BadParameter("Reverse tunnel validation error: empty domain name")
	}

	if len(r.DialAddrs) == 0 {
		return trace.BadParameter("Invalid dial address for reverse tunnel '%v'", r.DomainName)
	}

	for _, addr := range r.DialAddrs {
		_, err := utils.ParseAddr(addr)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

// CommandLabel is a label that has a value as a result of the
// output generated by running command, e.g. hostname
type CommandLabel struct {
	// Period is a time between command runs
	Period time.Duration `json:"period"`
	// Command is a command to run
	Command []string `json:"command"` //["/usr/bin/hostname", "--long"]
	// Result captures standard output
	Result string `json:"result"`
}

// CommandLabels is a set of command labels
type CommandLabels map[string]CommandLabel

// SetEnv sets the value of the label from environment variable
func (c *CommandLabels) SetEnv(v string) error {
	if err := json.Unmarshal([]byte(v), c); err != nil {
		return trace.Wrap(err, "Can't parse Command Labels")
	}
	return nil
}

// LabelsMap returns the full key:value map of both static labels and
// "command labels"
func (s *Server) LabelsMap() map[string]string {
	lmap := make(map[string]string)
	for key, value := range s.Labels {
		lmap[key] = value
	}
	for key, cmd := range s.CmdLabels {
		lmap[key] = cmd.Result
	}
	return lmap
}

// MatchAgainst takes a map of labels and returns True if this server
// has ALL of them
//
// Any server matches against an empty label set
func (s *Server) MatchAgainst(labels map[string]string) bool {
	if labels != nil {
		myLabels := s.LabelsMap()
		for key, value := range labels {
			if myLabels[key] != value {
				return false
			}
		}
	}
	return true
}

// LabelsString returns a comma separated string with all node's labels
func (s *Server) LabelsString() string {
	labels := []string{}
	for key, val := range s.Labels {
		labels = append(labels, fmt.Sprintf("%s=%s", key, val))
	}
	for key, val := range s.CmdLabels {
		labels = append(labels, fmt.Sprintf("%s=%s", key, val.Result))
	}
	sort.Strings(labels)
	return strings.Join(labels, ",")
}
