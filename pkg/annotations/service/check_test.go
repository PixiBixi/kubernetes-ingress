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

package service

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/haproxytech/client-native/v6/models"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
	"github.com/stretchr/testify/require"
)

// The once-key is process-global and tests reuse the process: fresh names per invocation.
var checkSeq atomic.Int64

func TestCheckDeprecationWarning(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	// "check" has a "true" default, so GetValue never returns empty: presence,
	// not value, decides that the user is using the annotation.
	require.NoError(t, NewCheck("check", &models.Backend{}).Process(store.K8s{}))
	require.Empty(t, buf.String(), "an annotation the user did not set must not warn")

	name := fmt.Sprintf("check-%d", checkSeq.Add(1))
	backend := &models.Backend{}
	require.NoError(t, NewCheck(name, backend).Process(store.K8s{}, map[string]string{name: "true"}))
	require.NotNil(t, backend.DefaultServer)
	require.Contains(t, buf.String(), fmt.Sprintf("DEPRECATED: annotation %q", name))
	require.Equal(t, 1, strings.Count(buf.String(), "DEPRECATED"), "warning must be logged once per app run")
}
