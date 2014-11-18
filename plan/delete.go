//  Copyright (c) 2014 Couchbase, Inc.
//  Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file
//  except in compliance with the License. You may obtain a copy of the License at
//    http://www.apache.org/licenses/LICENSE-2.0
//  Unless required by applicable law or agreed to in writing, software distributed under the
//  License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
//  either express or implied. See the License for the specific language governing permissions
//  and limitations under the License.

package plan

import (
	"encoding/json"
	"github.com/couchbaselabs/query/datastore"
)

type SendDelete struct {
	readwrite
	keyspace datastore.Keyspace
}

func NewSendDelete(keyspace datastore.Keyspace) *SendDelete {
	return &SendDelete{
		keyspace: keyspace,
	}
}

func (this *SendDelete) Accept(visitor Visitor) (interface{}, error) {
	return visitor.VisitSendDelete(this)
}

func (this *SendDelete) Keyspace() datastore.Keyspace {
	return this.keyspace
}

func (this *SendDelete) MarshalJSON() ([]byte, error) {
	r := map[string]interface{}{"#operator": "Delete"}
	r["keyspace"] = this.keyspace.Name()
	return json.Marshal(r)
}
