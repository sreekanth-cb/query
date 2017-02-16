//  Copyright (c) 2014 Couchbase, Inc.
//  Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file
//  except in compliance with the License. You may obtain a copy of the License at
//    http://www.apache.org/licenses/LICENSE-2.0
//  Unless required by applicable law or agreed to in writing, software distributed under the
//  License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
//  either express or implied. See the License for the specific language governing permissions
//  and limitations under the License.

package execution

import (
	"encoding/json"
	"math"
	"time"

	"github.com/couchbase/query/datastore"
	"github.com/couchbase/query/errors"
	"github.com/couchbase/query/logging"
	"github.com/couchbase/query/plan"
	"github.com/couchbase/query/value"
)

type PrimaryScan struct {
	base
	plan *plan.PrimaryScan
}

func NewPrimaryScan(plan *plan.PrimaryScan, context *Context) *PrimaryScan {
	rv := &PrimaryScan{
		base: newBase(context),
		plan: plan,
	}

	rv.output = rv
	return rv
}

func (this *PrimaryScan) Accept(visitor Visitor) (interface{}, error) {
	return visitor.VisitPrimaryScan(this)
}

func (this *PrimaryScan) Copy() Operator {
	return &PrimaryScan{this.base.copy(), this.plan}
}

func (this *PrimaryScan) RunOnce(context *Context, parent value.Value) {
	this.once.Do(func() {
		defer context.Recover() // Recover from any panic
		context.AddPhaseOperator(PRIMARY_SCAN)
		this.phaseTimes = func(d time.Duration) { context.AddPhaseTime(PRIMARY_SCAN, d) }
		defer close(this.itemChannel) // Broadcast that I have stopped
		defer this.notify()           // Notify that I have stopped

		this.scanPrimary(context, parent)
	})
}

func (this *PrimaryScan) scanPrimary(context *Context, parent value.Value) {
	this.switchPhase(_EXECTIME)
	defer this.switchPhase(_NOTIME)
	conn := this.newIndexConnection(context)
	defer notifyConn(conn.StopChannel()) // Notify index that I have stopped

	go this.scanEntries(context, conn)

	var entry, lastEntry *datastore.IndexEntry

	ok := true
	nitems := 0

	var docs uint64 = 0
	defer func() {
		if docs > 0 {
			context.AddPhaseCount(PRIMARY_SCAN, docs)
		}
	}()

	for ok {
		this.switchPhase(_SERVTIME)
		select {
		case <-this.stopChannel:
			return
		default:
		}

		select {
		case entry, ok = <-conn.EntryChannel():
			this.switchPhase(_EXECTIME)
			if ok {

				// current policy is to only count 'in' documents
				// from operators, not kv
				// add this.addInDocs(1) if this changes
				cv := value.NewScopeValue(make(map[string]interface{}), parent)
				av := value.NewAnnotatedValue(cv)
				av.SetAttachment("meta", map[string]interface{}{"id": entry.PrimaryKey})
				ok = this.sendItem(av)
				lastEntry = entry
				nitems++
				docs++
				if docs > _PHASE_UPDATE_COUNT {
					context.AddPhaseCount(PRIMARY_SCAN, docs)
					docs = 0
				}
			}

		case <-this.stopChannel:
			return
		}

	}

	if conn.Timeout() {
		logging.Errorp("Primary index scan timeout - resorting to chunked scan",
			logging.Pair{"chunkSize", nitems},
			logging.Pair{"startingEntry", lastEntry})
		if lastEntry == nil {
			// no key for chunked scans (primary scan returned 0 items)
			context.Error(errors.NewCbIndexScanTimeoutError(nil))
		}
		// do chunked scans; nitems gives the chunk size, and lastEntry the starting point
		for lastEntry != nil {
			lastEntry = this.scanPrimaryChunk(context, parent, nitems, lastEntry)
		}
	}
}

func (this *PrimaryScan) scanPrimaryChunk(context *Context, parent value.Value, chunkSize int, indexEntry *datastore.IndexEntry) *datastore.IndexEntry {
	this.switchPhase(_EXECTIME)
	defer this.switchPhase(_NOTIME)
	conn, _ := datastore.NewSizedIndexConnection(int64(chunkSize), context)
	conn.SetPrimary()
	defer notifyConn(conn.StopChannel()) // Notify index that I have stopped

	go this.scanChunk(context, conn, chunkSize, indexEntry)

	var entry, lastEntry *datastore.IndexEntry

	ok := true

	nitems := 0
	var docs uint64 = 0
	defer func() {
		if nitems > 0 {
			context.AddPhaseCount(PRIMARY_SCAN, docs)
		}
	}()

	for ok {
		this.switchPhase(_SERVTIME)
		select {
		case <-this.stopChannel:
			return nil
		default:
		}

		select {
		case entry, ok = <-conn.EntryChannel():
			this.switchPhase(_EXECTIME)
			if ok {
				cv := value.NewScopeValue(make(map[string]interface{}), parent)
				av := value.NewAnnotatedValue(cv)
				av.SetAttachment("meta", map[string]interface{}{"id": entry.PrimaryKey})
				ok = this.sendItem(av)
				lastEntry = entry
				nitems++
				docs++
				if docs > _PHASE_UPDATE_COUNT {
					context.AddPhaseCount(PRIMARY_SCAN, docs)
					docs = 0
				}
			}

		case <-this.stopChannel:
			return nil
		}

	}
	logging.Debugp("Primary index chunked scan", logging.Pair{"chunkSize", nitems}, logging.Pair{"lastKey", lastEntry})
	return lastEntry
}

func (this *PrimaryScan) scanEntries(context *Context, conn *datastore.IndexConnection) {
	defer context.Recover() // Recover from any panic

	limit := int64(math.MaxInt64)
	if this.plan.Limit() != nil {
		lv, err := this.plan.Limit().Evaluate(nil, context)
		if err == nil && lv.Type() == value.NUMBER {
			limit = int64(lv.Actual().(float64))
		}
	}

	keyspace := this.plan.Keyspace()
	scanVector := context.ScanVectorSource().ScanVector(keyspace.NamespaceId(), keyspace.Name())

	index := this.plan.Index()
	this.switchPhase(_SERVTIME)
	index.ScanEntries(context.RequestId(), limit,
		context.ScanConsistency(), scanVector, conn)
	this.switchPhase(_EXECTIME)
}

func (this *PrimaryScan) scanChunk(context *Context, conn *datastore.IndexConnection, chunkSize int, indexEntry *datastore.IndexEntry) {
	defer context.Recover() // Recover from any panic
	ds := &datastore.Span{}
	// do the scan starting from, but not including, the given index entry:
	ds.Range = datastore.Range{
		Inclusion: datastore.NEITHER,
		Low:       []value.Value{value.NewValue(indexEntry.PrimaryKey)},
	}
	keyspace := this.plan.Keyspace()
	scanVector := context.ScanVectorSource().ScanVector(keyspace.NamespaceId(), keyspace.Name())
	this.switchPhase(_SERVTIME)
	this.plan.Index().Scan(context.RequestId(), ds, true, int64(chunkSize),
		context.ScanConsistency(), scanVector, conn)
	this.switchPhase(_EXECTIME)
}

func (this *PrimaryScan) newIndexConnection(context *Context) *datastore.IndexConnection {
	var conn *datastore.IndexConnection

	// Use keyspace count to create a sized index connection
	keyspace := this.plan.Keyspace()
	size, err := keyspace.Count(context)
	if err == nil {
		if size <= 0 {
			size = 1
		}

		conn, err = datastore.NewSizedIndexConnection(size, context)
		conn.SetPrimary()
	}

	// Use non-sized API and log error
	if err != nil {
		conn = datastore.NewIndexConnection(context)
		conn.SetPrimary()
		logging.Errorp("PrimaryScan.newIndexConnection ", logging.Pair{"error", err})
	}

	return conn
}

func (this *PrimaryScan) MarshalJSON() ([]byte, error) {
	r := this.plan.MarshalBase(func(r map[string]interface{}) {
		this.marshalTimes(r)
	})
	return json.Marshal(r)
}
