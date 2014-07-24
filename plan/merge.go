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
	"github.com/couchbaselabs/query/algebra"
	"github.com/couchbaselabs/query/datastore"
	"github.com/couchbaselabs/query/expression"
)

type Merge struct {
	keyspace datastore.Keyspace
	ref      *algebra.KeyspaceRef
	key      expression.Expression
	update   Operator
	delete   Operator
	insert   Operator
	alias    string
}

func NewMerge(keyspace datastore.Keyspace, ref *algebra.KeyspaceRef,
	key expression.Expression, update, delete, insert Operator) *Merge {
	return &Merge{keyspace, ref, key, update, delete, insert, ""}
}

func (this *Merge) Accept(visitor Visitor) (interface{}, error) {
	return visitor.VisitMerge(this)
}

func (this *Merge) Keyspace() datastore.Keyspace {
	return this.keyspace
}

func (this *Merge) Key() expression.Expression {
	return this.key
}

func (this *Merge) Update() Operator {
	return this.update
}

func (this *Merge) Delete() Operator {
	return this.delete
}

func (this *Merge) Insert() Operator {
	return this.insert
}

func (this *Merge) Alias() string {
	return this.alias
}
