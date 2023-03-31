/*

 Copyright 2023 Gravitational, Inc.

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

package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"

	"github.com/gogo/protobuf/proto"
	"github.com/gorilla/websocket"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/web"
	"github.com/gravitational/trace"
)

const (
	cookie = "__Host-grv_csrf=54034323553f2d32b48e39164cf72ea11505ffff951d82ba2e103bf873ce3f5d; __Host-session=7b2275736572223a22626f62222c22736964223a2262336662313161643634616663666536383862653264303537316633353761653664623565636365333231333636386164393234353435633630653434333862227d"
	auth   = "1e9529ae1bd4b27cc5a2a1f34db792d157d8c763f7d9d8e6e0b948a3e72474b0"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	u := url.URL{
		Host:   "example.com:3080",
		Scheme: client.WSS,
		Path:   "/v1/webapi/command/example.com/execute",
	}

	requestData := web.CommandRequest{
		Command: "ls -lah",
		Login:   "jnyckowski",
		Labels: map[string]string{
			"env": "dev",
		},
		NodeID: "854e9299-c604-4af8-baa9-2580c4337a84",
	}

	data, err := json.Marshal(requestData)
	if err != nil {
		log.Fatal(err)
	}

	q := u.Query()
	q.Set("params", string(data))
	u.RawQuery = q.Encode()

	dialer := websocket.Dialer{}
	dialer.TLSClientConfig = &tls.Config{
		InsecureSkipVerify: true,
	}

	header := http.Header{}
	header.Add("Origin", "https://example.com")
	header.Add("Authorization", "Bearer "+auth)
	header.Add("Cookie", cookie)

	ws, resp, err := dialer.Dial(u.String(), header)
	if err != nil {
		log.Fatal(trace.Wrap(err))
	}

	defer func() {
		ws.Close()
		resp.Body.Close()
	}()

	type payloadEnv struct {
		NodeID  string `json:"node_id"`
		Type    string `json:"type"`
		Payload []byte `json:"payload"`
	}
	for {
		ty, raw, err := ws.ReadMessage()
		if err != nil {
			if err == io.EOF {
				break
			}
			log.Fatal(err)
		}

		env := web.Envelope{}
		if err := proto.Unmarshal(raw, &env); err != nil {
			log.Fatal(err)
		}

		p := &payloadEnv{}
		if err := json.Unmarshal([]byte(env.Payload), p); err != nil {
			log.Print(err)
			continue
		}

		fmt.Printf("%v %v %s %s\n%s\n", ty, err, p.NodeID, p.Type, string(p.Payload))
	}
}
