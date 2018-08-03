//  Copyright (c) 2018 Couchbase, Inc.
//  Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file
//  except in compliance with the License. You may obtain a copy of the License at
//    http://www.apache.org/licenses/LICENSE-2.0
//  Unless required by applicable law or agreed to in writing, software distributed under the
//  License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
//  either express or implied. See the License for the specific language governing permissions
//  and limitations under the License.

package algebra

import (
	"github.com/couchbase/query/expression"
	"github.com/couchbase/query/value"
)

/*
This represents the Aggregate function  VARIANCE_POP(expr). It returns
the arithmetic population variance of all the number values in the
group. Type VarPop is a struct that inherits from AggregateBase.
*/
type VarPop struct {
	AggregateBase
}

/*
The function NewVarPop calls NewAggregateBase to
create an aggregate function named var_pop with
one expression as input.
*/
func NewVarPop(operand expression.Expression) Aggregate {
	rv := &VarPop{
		*NewAggregateBase("var_pop", operand),
	}

	rv.SetExpr(rv)
	return rv
}

/*
It calls the VisitFunction method by passing in the receiver to
and returns the interface. It is a visitor pattern.
*/
func (this *VarPop) Accept(visitor expression.Visitor) (interface{}, error) {
	return visitor.VisitFunction(this)
}

/*
It returns a value of type NUMBER.
*/
func (this *VarPop) Type() value.Type {
	return value.NUMBER
}

/*
Calls the evaluate method for aggregate functions and passes in the
receiver, current item and current context.
*/
func (this *VarPop) Evaluate(item value.Value, context expression.Context) (value.Value, error) {
	return this.evaluate(this, item, context)
}

/*
The constructor returns a NewVarPop with the input operand
cast to a Function as the FunctionConstructor.
*/
func (this *VarPop) Constructor() expression.FunctionConstructor {
	return func(operands ...expression.Expression) expression.Function {
		return NewVarPop(operands[0])
	}
}

/*
If no input to the VarPop function, then the default value
returned is a null.
*/
func (this *VarPop) Default() value.Value {
	return value.NULL_VALUE
}

/*
Aggregates input data by evaluating operands.
For all values other than Number, return the input value itself.
Maintain two variables for sum and
list of all the values of type NUMBER.
Call addStddevVariance to compute the intermediate aggregate value and return it.
*/
func (this *VarPop) CumulateInitial(item, cumulative value.Value, context Context) (value.Value, error) {
	item, e := this.Operand().Evaluate(item, context)
	if e != nil {
		return nil, e
	}

	if item.Type() != value.NUMBER {
		return cumulative, nil
	}

	return addStddevVariance(item, cumulative, false)
}

/*
Aggregates intermediate results and return them.
*/
func (this *VarPop) CumulateIntermediate(part, cumulative value.Value, context Context) (value.Value, error) {
	return cumulateStddevVariance(part, cumulative, false)
}

/*
Compute the population variance as the final.
Return NULL if no values of type NUMBER exist.
Return zero if only one value exists.
calculate population variance according to definition
and return it.
*/
func (this *VarPop) ComputeFinal(cumulative value.Value, context Context) (value.Value, error) {
	if cumulative == value.NULL_VALUE {
		return cumulative, nil
	}

	variance, e := computeVariance(cumulative, false, false, 0.0)
	if e != nil {
		return nil, e
	}

	return variance, nil
}
