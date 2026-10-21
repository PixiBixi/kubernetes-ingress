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
var deprecationSeq atomic.Int64

func TestBackendAnnotationsWithoutFaithfulReplacement(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	// Secrets the controller resolves and refreshes: no Backend resource field can carry them.
	caName := fmt.Sprintf("server-ca-%d", deprecationSeq.Add(1))
	require.NoError(t, NewCA(caName, nil, &models.Backend{}).Process(store.K8s{}, map[string]string{caName: "ns/ca"}))
	crtName := fmt.Sprintf("server-crt-%d", deprecationSeq.Add(1))
	require.NoError(t, NewCrt(crtName, nil, &models.Backend{}).Process(store.K8s{}, map[string]string{crtName: "ns/crt"}))

	// A per-server budget divided by the replica count: a Backend resource value applies per instance.
	maxconnName := fmt.Sprintf("pod-maxconn-%d", deprecationSeq.Add(1))
	backend := &models.Backend{}
	require.NoError(t, NewMaxconn(maxconnName, backend).Process(store.K8s{}, map[string]string{maxconnName: "10"}))
	require.NotNil(t, backend.DefaultServer)

	out := buf.String()
	require.Equal(t, 3, strings.Count(out, "DEPRECATED"), "each annotation must warn once")
	require.NotContains(t, out, "custom resource", "no Backend resource can replace these")
}

func TestTimeoutServerNoticeScope(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	// ConfigMap-only value: one cluster-wide default, the Defaults notice owns it.
	cfgOnly := fmt.Sprintf("timeout-server-%d", deprecationSeq.Add(1))
	require.NoError(t, NewTimeoutServer(cfgOnly, &models.Backend{}).Process(store.K8s{}, map[string]string{cfgOnly: "30s"}))
	require.Empty(t, buf.String(), "a ConfigMap value must not name the per-service Backend resource")

	// Service scope: names the resource that replaces it.
	svcScoped := fmt.Sprintf("timeout-server-%d", deprecationSeq.Add(1))
	require.NoError(t, NewTimeoutServer(svcScoped, &models.Backend{}).
		Process(store.K8s{}, map[string]string{svcScoped: "40s"}, map[string]string{}))
	require.Contains(t, buf.String(), "use the Backend custom resource instead")
}
