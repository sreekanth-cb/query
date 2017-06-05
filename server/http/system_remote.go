//  Copyright (c) 2016 Couchbase, Inc.
//  Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file
//  except in compliance with the License. You may obtain a copy of the License at
//    http://www.apache.org/licenses/LICENSE-2.0
//  Unless required by applicable law or agreed to in writing, software distributed under the
//  License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
//  either express or implied. See the License for the specific language governing permissions
//  and limitations under the License.

// This implements remote system keyspace access for the REST based http package

// +build enterprise

package http

import (
	"bytes"
	"encoding/json"
	goErr "errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/couchbase/cbauth"
	"github.com/couchbase/query/clustering"
	"github.com/couchbase/query/distributed"
	"github.com/couchbase/query/errors"
)

// http implementation of SystemRemoteAccess
type systemRemoteHttp struct {
	state       clustering.Mode
	configStore clustering.ConfigurationStore
}

func NewSystemRemoteAccess(cfgStore clustering.ConfigurationStore) distributed.SystemRemoteAccess {
	return &systemRemoteHttp{
		configStore: cfgStore,
	}
}

// construct a key from node name and local key
func (this *systemRemoteHttp) MakeKey(node string, key string) string {
	if node == "" {
		return key
	} else {
		return "[" + node + "]" + key
	}
}

// split global key into name and local key
func (this *systemRemoteHttp) SplitKey(key string) (string, string) {
	if strings.HasPrefix(key, "[") {
		fields := strings.FieldsFunc(key, func(c rune) bool {
			return c == '[' || c == ']'
		})
		if len(fields) == 2 {
			return fields[0], fields[1]
		}
	}
	return "", key
}

// get remote keys from the specified nodes for the specified endpoint
func (this *systemRemoteHttp) GetRemoteKeys(nodes []string, endpoint string,
	keyFn func(id string) bool, warnFn func(warn errors.Error)) {
	var keys []string

	// now that the local node name can change, use a consistent one across the scan
	whoAmI := this.WhoAmI()

	// not part of a cluster, no keys can be gathered
	if len(whoAmI) == 0 {
		return
	}

	// no nodes means all nodes
	if len(nodes) == 0 {

		cm := this.configStore.ConfigurationManager()
		clusters, err := cm.GetClusters()
		if err != nil {
			if warnFn != nil {
				warnFn(errors.NewSystemRemoteWarning(err, "scan", endpoint))
			}
			return
		}

		for _, c := range clusters {
			clm := c.ClusterManager()
			queryNodes, err := clm.GetQueryNodes()
			if err != nil {
				if warnFn != nil {
					warnFn(errors.NewSystemRemoteWarning(err, "scan", endpoint))
				}
				continue
			}

			for _, queryNode := range queryNodes {
				node := queryNode.Name()

				// skip ourselves, we will be processed locally
				if node == whoAmI {
					continue
				}

				body, opErr := doRemoteOp(queryNode, "indexes/"+endpoint, "GET", distributed.NO_CREDS, "")
				if opErr != nil {
					if warnFn != nil {
						warnFn(errors.NewSystemRemoteWarning(opErr, "scan", endpoint))
					}
					continue
				}

				jErr := json.Unmarshal(body, &keys)
				if jErr != nil {
					if warnFn != nil {
						warnFn(errors.NewSystemRemoteWarning(jErr, "scan", endpoint))
					}
					continue
				}

				if keyFn != nil {
					for _, key := range keys {
						if !keyFn("[" + node + "]" + key) {
							return
						}
					}
				}
			}
		}
	} else {

		for _, node := range nodes {

			// skip ourselves, it will be processed locally
			if node == whoAmI {
				continue
			}

			queryNode, err := getQueryNode(this.configStore, node, "scan", endpoint)
			if err != nil {
				if warnFn != nil {
					warnFn(err)
				}
				continue
			}

			body, opErr := doRemoteOp(queryNode, "indexes/"+endpoint, "GET", distributed.NO_CREDS, "")
			if opErr != nil {
				if warnFn != nil {
					warnFn(errors.NewSystemRemoteWarning(opErr, "scan", endpoint))
				}
				continue
			}
			jErr := json.Unmarshal(body, &keys)
			if jErr != nil {
				if warnFn != nil {
					warnFn(errors.NewSystemRemoteWarning(jErr, "scan", endpoint))
				}
				continue
			}
			if keyFn != nil {
				for _, key := range keys {
					if !keyFn("[" + node + "]" + key) {
						return
					}
				}
			}
		}
	}
}

// get a specified remote document from a remote node
func (this *systemRemoteHttp) GetRemoteDoc(node string, key string, endpoint string, command string,
	docFn func(map[string]interface{}), warnFn func(warn errors.Error), creds distributed.Creds, authToken string) {
	var body []byte
	var doc map[string]interface{}

	queryNode, err := getQueryNode(this.configStore, node, "fetch", endpoint)
	if err != nil {
		if warnFn != nil {
			warnFn(err)
		}
		return
	}

	body, opErr := doRemoteOp(queryNode, endpoint+"/"+key, command, creds, authToken)
	if opErr != nil {
		if warnFn != nil {
			warnFn(errors.NewSystemRemoteWarning(opErr, "fetch", endpoint))
		}
		return
	}

	jErr := json.Unmarshal(body, &doc)
	if jErr != nil {
		if warnFn != nil {
			errors.NewSystemRemoteWarning(jErr, "fetch", endpoint)
		}
		return
	}

	if docFn != nil {
		docFn(doc)
	}
}

func credsAsJSON(creds distributed.Creds) string {
	buf := new(bytes.Buffer)
	buf.WriteString("[")
	var num = 0
	for k, v := range creds {
		if num > 0 {
			buf.WriteString(",")
		}
		buf.WriteString("{")
		buf.WriteString("\"user\":\"")
		buf.WriteString(k)
		buf.WriteString("\",\"pass\":\"")
		buf.WriteString(v)
		buf.WriteString("\"}")
		num++
	}
	buf.WriteString("]")
	return buf.String()
}

// helper for the REST op
func doRemoteOp(node clustering.QueryNode, endpoint string, command string, creds distributed.Creds, authToken string) ([]byte, error) {
	var HTTPTransport = &http.Transport{MaxIdleConnsPerHost: 10} //MaxIdleConnsPerHost}
	var HTTPClient = &http.Client{Transport: HTTPTransport}

	if node == nil {
		return nil, goErr.New("missing remote node")
	}

	numCredentials := len(creds)
	fullEndpoint := node.ClusterEndpoint() + "/" + endpoint
	if numCredentials > 1 {
		fullEndpoint += "?creds=" + credsAsJSON(creds)
	}
	authenticator := cbauth.Default

	// Here, I'm leveraging the fact that the node name is the host:port of the mgmt
	// endpoint associated with the node. This is the same hostport pair that allows us
	// to access the admin creds for that node.
	u, p, _ := authenticator.GetHTTPServiceAuth(node.Name())
	request, _ := http.NewRequest(command, fullEndpoint, nil)
	request.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	request.SetBasicAuth(u, p)

	resp, err := HTTPClient.Do(request)

	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		_, _ = ioutil.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, err
	}

	return ioutil.ReadAll(resp.Body)
}

// helper to map a node name to a node
func getQueryNode(configStore clustering.ConfigurationStore, node string, op string, endpoint string) (clustering.QueryNode, errors.Error) {
	if configStore == nil {
		return nil, errors.NewSystemRemoteWarning(goErr.New("missing config store"), op, endpoint)
	}

	cm := configStore.ConfigurationManager()
	clusters, err := cm.GetClusters()
	if err != nil {
		return nil, err
	}

	for _, c := range clusters {
		clm := c.ClusterManager()
		queryNodes, err := clm.GetQueryNodes()
		if err != nil {
			continue
		}

		for _, queryNode := range queryNodes {
			if queryNode.Name() == node {
				return queryNode, nil
			}
		}
	}
	return nil, errors.NewSystemRemoteWarning(fmt.Errorf("node %v not found", node), op, endpoint)
}

// returns the local node identity, as known to the cluster
func (this *systemRemoteHttp) WhoAmI() string {

	// when clustered operations begin, we'll determine
	// if we are part of a cluster.
	// if yes, we'll refresh our node name from clustering
	// at every call, if not, we turn off clustering for good
	if this.state == "" {

		// not part of a cluster if there isn't a configStore
		if this.configStore == nil {
			this.state = clustering.STANDALONE
			return ""
		}

		state, err := this.configStore.State()
		if err != nil {
			this.state = clustering.STANDALONE
			return ""
		}
		this.state = state

		if this.state == clustering.STANDALONE {
			return ""
		}

		// not part of a cluster if we can't work out our own name
		localNode, err := this.configStore.WhoAmI()
		if err != nil {
			this.state = clustering.STANDALONE
			return ""
		}
		return localNode
	} else if this.state == clustering.STANDALONE {
		return ""
	}

	localNode, _ := this.configStore.WhoAmI()
	return localNode
}

func (this *systemRemoteHttp) GetNodeNames() []string {
	var names []string

	if len(this.WhoAmI()) == 0 {
		return names
	}
	cm := this.configStore.ConfigurationManager()
	clusters, err := cm.GetClusters()
	if err != nil {
		return names
	}

	for _, c := range clusters {
		clm := c.ClusterManager()
		queryNodes, err := clm.GetQueryNodes()
		if err != nil {
			continue
		}

		for _, queryNode := range queryNodes {
			names = append(names, queryNode.Name())
		}
	}
	return names
}
