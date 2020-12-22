/*
Copyright 2019-2020 vChain, Inc.

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

package immuc_test

import (
	"os"
	"strings"
	"testing"

	"github.com/codenotary/immudb/pkg/client"

	test "github.com/codenotary/immudb/cmd/immuclient/immuclienttest"
	"github.com/codenotary/immudb/pkg/server"
	"github.com/codenotary/immudb/pkg/server/servertest"
)

func TestHistory(t *testing.T) {
	options := server.DefaultOptions().WithAuth(true)
	bs := servertest.NewBufconnServer(options)

	go func() { bs.Start() }()

	defer os.RemoveAll(options.Dir)

	ts := client.NewTokenService().WithTokenFileName("testTokenFile").WithHds(&test.HomedirServiceMock{})
	ic := test.NewClientTest(&test.PasswordReader{
		Pass: []string{"immudb"},
	}, ts)
	ic.Connect(bs.Dialer)
	ic.Login("immudb")

	msg, err := ic.Imc.History([]string{"key"})
	if err != nil {
		t.Fatal("History fail", err)
	}
	if !strings.Contains(msg, "No item found") {
		t.Fatalf("History fail %s", msg)
	}

	_, err = ic.Imc.Set([]string{"key", "value"})
	if err != nil {
		t.Fatal("History fail", err)
	}
	msg, err = ic.Imc.History([]string{"key"})
	if err != nil {
		t.Fatal("History fail", err)
	}
	if !strings.Contains(msg, "hash") {
		t.Fatalf("History fail %s", msg)
	}
}
