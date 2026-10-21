// Copyright 2019 HAProxy Technologies LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ingress

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/maps"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/rules"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
)

// The once-keys are process-global and tests reuse the process: fresh names per invocation.
var accessControlSeq atomic.Int64

func TestAccessControlDeprecationNotices(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	// The current name gets the generic removal notice, once.
	denyListName := fmt.Sprintf("deny-list-%d", accessControlSeq.Add(1))
	ann := NewDenyList(denyListName, &rules.List{}, newRecordingMaps())
	require.NoError(t, ann.Process(store.K8s{}, map[string]string{denyListName: "192.168.1.1"}))
	require.Contains(t, buf.String(), fmt.Sprintf("DEPRECATED: annotation %q will be removed", denyListName))
	require.Equal(t, 1, strings.Count(buf.String(), "DEPRECATED"), "warning must be logged once per app run")

	// The deprecated alias warns about the rename only: its fallback must not
	// also mark the new name as used and double-notice the same user.
	buf.Reset()
	aliasName := fmt.Sprintf("deny-list-%d", accessControlSeq.Add(1))
	rulesList := &rules.List{}
	ann = NewDenyList(aliasName, rulesList, newRecordingMaps())
	require.NoError(t, ann.Process(store.K8s{}, map[string]string{"blacklist": "192.168.1.1"}))
	require.Len(t, *rulesList, 1, "the deprecated alias must keep working")
	require.NotContains(t, buf.String(), fmt.Sprintf("annotation %q will be removed", aliasName))
}

// recordingMaps implements maps.Maps and records appended rows so tests can
// assert exactly which addresses end up in the access-control map file.
type recordingMaps struct {
	entries map[maps.Name][]string
}

func newRecordingMaps() *recordingMaps {
	return &recordingMaps{entries: make(map[maps.Name][]string)}
}

func (m *recordingMaps) MapAppend(name maps.Name, row string) {
	if row == "" {
		return
	}
	m.entries[name] = append(m.entries[name], row)
}

func (m *recordingMaps) MapExists(name maps.Name) bool {
	return len(m.entries[name]) != 0
}

func (m *recordingMaps) RefreshMaps(api.HAProxyClient) {}

func (m *recordingMaps) CleanMaps() {}

// allEntries returns the rows of all recorded maps. Process writes to a
// single hash-named map, so this is the content of that map.
func (m *recordingMaps) allEntries() []string {
	var rows []string
	for _, e := range m.entries {
		rows = append(rows, e...)
	}
	return rows
}

// TestAccessControl_Process tests the allow-list/deny-list annotation processing.
// It validates that:
//   - Single IP addresses and CIDR ranges are correctly parsed and stored in a map file
//   - Multiple comma-separated values are supported
//   - Empty elements produced by a trailing comma, consecutive commas or
//     surrounding whitespace are skipped instead of invalidating the whole list
//   - A list without any valid element is still rejected (an empty allow-list
//     map would deny all traffic)
//   - Invalid addresses are rejected with an appropriate error message
//   - Pattern file references (using "patterns/" prefix) are handled correctly
//
//revive:disable-next-line:function-length
func TestAccessControl_Process(t *testing.T) {
	tests := []struct {
		name        string
		annotation  string
		value       string
		denyList    bool
		wantErr     bool
		wantEntries []string
		wantMapPath string
	}{
		{
			name:        "single IP",
			annotation:  "allow-list",
			value:       "192.168.1.1",
			wantEntries: []string{"192.168.1.1"},
		},
		{
			name:        "multiple IPs and CIDRs",
			annotation:  "allow-list",
			value:       "192.168.1.1, 10.0.0.0/8, 172.16.0.0/12",
			wantEntries: []string{"192.168.1.1", "10.0.0.0/8", "172.16.0.0/12"},
		},
		{
			name:        "trailing comma",
			annotation:  "allow-list",
			value:       "192.168.1.0/24,",
			wantEntries: []string{"192.168.1.0/24"},
		},
		{
			name:        "consecutive commas",
			annotation:  "allow-list",
			value:       "192.168.1.1,,10.0.0.0/8",
			wantEntries: []string{"192.168.1.1", "10.0.0.0/8"},
		},
		{
			name:        "leading comma and spaces around elements",
			annotation:  "allow-list",
			value:       " , 192.168.1.1 ,  10.0.0.0/8  ",
			wantEntries: []string{"192.168.1.1", "10.0.0.0/8"},
		},
		{
			name:        "deprecated whitelist alias with trailing comma",
			annotation:  "whitelist",
			value:       "10.0.0.0/16,",
			wantEntries: []string{"10.0.0.0/16"},
		},
		{
			name:        "deny-list with trailing comma",
			annotation:  "deny-list",
			value:       "192.168.1.1,",
			denyList:    true,
			wantEntries: []string{"192.168.1.1"},
		},
		{
			name:       "only commas",
			annotation: "allow-list",
			value:      ",,",
			wantErr:    true,
		},
		{
			name:       "only spaces",
			annotation: "allow-list",
			value:      "   ",
			wantErr:    true,
		},
		{
			name:       "empty value",
			annotation: "allow-list",
			value:      "",
		},
		{
			name:       "invalid address",
			annotation: "allow-list",
			value:      "192.168.1.1, invalid",
			wantErr:    true,
		},
		{
			name:       "invalid CIDR",
			annotation: "allow-list",
			value:      "192.168.1.0/33",
			wantErr:    true,
		},
		{
			name:        "patterns file",
			annotation:  "allow-list",
			value:       "patterns/allowlist",
			wantMapPath: "patterns/allowlist",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockMaps := newRecordingMaps()
			rulesList := &rules.List{}

			var accessControl *AccessControl
			if tt.denyList {
				accessControl = NewDenyList(tt.annotation, rulesList, mockMaps)
			} else {
				accessControl = NewAllowList(tt.annotation, rulesList, mockMaps)
			}

			err := accessControl.Process(store.K8s{}, map[string]string{tt.annotation: tt.value})

			if tt.wantErr {
				assert.Error(t, err)
				assert.Empty(t, *rulesList)
				return
			}

			require.NoError(t, err)

			if tt.value == "" {
				assert.Empty(t, *rulesList)
				return
			}

			require.Len(t, *rulesList, 1)
			reqDeny, ok := (*rulesList)[0].(*rules.ReqDeny)
			require.True(t, ok)
			assert.Equal(t, !tt.denyList, reqDeny.AllowList)

			if tt.wantMapPath != "" {
				assert.Equal(t, maps.Path(tt.wantMapPath), reqDeny.SrcIPsMap)
				return
			}

			assert.ElementsMatch(t, tt.wantEntries, mockMaps.allEntries())
		})
	}
}
