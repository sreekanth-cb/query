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
	"fmt"
	"time"

	"github.com/couchbase/query/errors"
	"github.com/couchbase/query/plan"
	"github.com/couchbase/query/value"
)

type SendInsert struct {
	base
	plan  *plan.SendInsert
	limit int64
}

func NewSendInsert(plan *plan.SendInsert, context *Context) *SendInsert {
	rv := &SendInsert{
		base:  newBase(context),
		plan:  plan,
		limit: -1,
	}

	rv.output = rv
	return rv
}

func (this *SendInsert) Accept(visitor Visitor) (interface{}, error) {
	return visitor.VisitSendInsert(this)
}

func (this *SendInsert) Copy() Operator {
	return &SendInsert{this.base.copy(), this.plan, this.limit}
}

func (this *SendInsert) RunOnce(context *Context, parent value.Value) {
	this.phaseTimes = func(d time.Duration) { context.AddPhaseTime(INSERT, d) }
	this.runConsumer(this, context, parent)
}

func (this *SendInsert) processItem(item value.AnnotatedValue, context *Context) bool {
	rv := this.limit != 0 && this.enbatch(item, this, context)

	if this.limit > 0 {
		this.limit--
	}

	return rv
}

func (this *SendInsert) beforeItems(context *Context, parent value.Value) bool {
	if this.plan.Limit() == nil {
		return true
	}

	limit, err := this.plan.Limit().Evaluate(parent, context)
	if err != nil {
		context.Error(errors.NewEvaluationError(err, "LIMIT clause"))
		return false
	}

	switch l := limit.Actual().(type) {
	case float64:
		this.limit = int64(l)
	default:
		context.Error(errors.NewInvalidValueError(fmt.Sprintf("Invalid LIMIT %v of type %T.", l, l)))
		return false
	}

	return true
}

func (this *SendInsert) afterItems(context *Context) {
	this.flushBatch(context)
}

func (this *SendInsert) flushBatch(context *Context) bool {
	defer this.releaseBatch(context)

	if len(this.batch) == 0 {
		return true
	}

	var dpairs []value.Pair
	if _INSERT_POOL.Size() >= len(this.batch) {
		dpairs = _INSERT_POOL.Get()
		defer _INSERT_POOL.Put(dpairs)
	} else {
		dpairs = make([]value.Pair, 0, len(this.batch))
	}

	keyExpr := this.plan.Key()
	valExpr := this.plan.Value()
	var key, val value.Value
	var err error
	var ok bool
	i := 0

	for _, av := range this.batch {
		dpairs = dpairs[0 : i+1]
		dpair := &dpairs[i]

		if keyExpr != nil {
			// INSERT ... SELECT
			key, err = keyExpr.Evaluate(av, context)
			if err != nil {
				context.Error(errors.NewEvaluationError(err,
					fmt.Sprintf("INSERT key for %v", av.GetValue())))
				continue
			}

			if valExpr != nil {
				val, err = valExpr.Evaluate(av, context)
				if err != nil {
					context.Error(errors.NewEvaluationError(err,
						fmt.Sprintf("INSERT value for %v", av.GetValue())))
					continue
				}
			} else {
				val = av
			}
		} else {
			// INSERT ... VALUES
			key, ok = av.GetAttachment("key").(value.Value)
			if !ok {
				context.Error(errors.NewInsertKeyError(av.GetValue()))
				continue
			}

			val, ok = av.GetAttachment("value").(value.Value)
			if !ok {
				context.Error(errors.NewInsertValueError(av.GetValue()))
				continue
			}
		}

		dpair.Name, ok = key.Actual().(string)
		if !ok {
			context.Error(errors.NewInsertKeyTypeError(key))
			continue
		}

		dpair.Value = val
		i++
	}

	dpairs = dpairs[0:i]

	this.switchPhase(_SERVTIME)

	// Perform the actual INSERT
	var er errors.Error
	dpairs, er = this.plan.Keyspace().Insert(dpairs)

	this.switchPhase(_EXECTIME)

	// Update mutation count with number of inserted docs
	context.AddMutationCount(uint64(len(dpairs)))

	if er != nil {
		context.Error(er)
	}

	// Capture the inserted keys in case there is a RETURNING clause
	for _, dp := range dpairs {
		dv := value.NewAnnotatedValue(dp.Value)
		dv.SetAttachment("meta", map[string]interface{}{"id": dp.Name})
		av := value.NewAnnotatedValue(make(map[string]interface{}, 1))
		av.SetAnnotations(dv)
		av.SetField(this.plan.Alias(), dv)
		if !this.sendItem(av) {
			return false
		}
	}

	return true
}

func (this *SendInsert) readonly() bool {
	return false
}

func (this *SendInsert) MarshalJSON() ([]byte, error) {
	r := this.plan.MarshalBase(func(r map[string]interface{}) {
		this.marshalTimes(r)
	})
	return json.Marshal(r)
}

var _INSERT_POOL = value.NewPairPool(_BATCH_SIZE)
