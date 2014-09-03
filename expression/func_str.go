//  Copyright (c) 2014 Couchbase, Inc.
//  Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file
//  except in compliance with the License. You may obtain a copy of the License at
//    http://www.apache.org/licenses/LICENSE-2.0
//  Unless required by applicable law or agreed to in writing, software distributed under the
//  License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
//  either express or implied. See the License for the specific language governing permissions
//  and limitations under the License.

package expression

import (
	"math"
	"strings"

	"github.com/couchbaselabs/query/value"
)

///////////////////////////////////////////////////
//
// Contains
//
///////////////////////////////////////////////////

type Contains struct {
	BinaryFunctionBase
}

func NewContains(first, second Expression) Function {
	return &Contains{
		*NewBinaryFunctionBase("contains", first, second),
	}
}

func (this *Contains) Accept(visitor Visitor) (interface{}, error) {
	return visitor.VisitFunction(this)
}

func (this *Contains) Type() value.Type { return value.BOOLEAN }

func (this *Contains) Evaluate(item value.Value, context Context) (value.Value, error) {
	return this.BinaryEval(this, item, context)
}

func (this *Contains) Apply(context Context, first, second value.Value) (value.Value, error) {
	if first.Type() == value.MISSING || second.Type() == value.MISSING {
		return value.MISSING_VALUE, nil
	} else if first.Type() != value.STRING || second.Type() != value.STRING {
		return value.NULL_VALUE, nil
	}

	rv := strings.Contains(first.Actual().(string), second.Actual().(string))
	return value.NewValue(rv), nil
}

func (this *Contains) Constructor() FunctionConstructor {
	return func(operands ...Expression) Function {
		return NewContains(operands[0], operands[1])
	}
}

///////////////////////////////////////////////////
//
// Length
//
///////////////////////////////////////////////////

type Length struct {
	UnaryFunctionBase
}

func NewLength(operand Expression) Function {
	return &Length{
		*NewUnaryFunctionBase("length", operand),
	}
}

func (this *Length) Accept(visitor Visitor) (interface{}, error) {
	return visitor.VisitFunction(this)
}

func (this *Length) Type() value.Type { return value.NUMBER }

func (this *Length) Evaluate(item value.Value, context Context) (value.Value, error) {
	return this.UnaryEval(this, item, context)
}

func (this *Length) Apply(context Context, arg value.Value) (value.Value, error) {
	if arg.Type() == value.MISSING {
		return value.MISSING_VALUE, nil
	} else if arg.Type() != value.STRING {
		return value.NULL_VALUE, nil
	}

	rv := len(arg.Actual().(string))
	return value.NewValue(float64(rv)), nil
}

func (this *Length) Constructor() FunctionConstructor {
	return func(operands ...Expression) Function {
		return NewLength(operands[0])
	}
}

///////////////////////////////////////////////////
//
// Lower
//
///////////////////////////////////////////////////

type Lower struct {
	UnaryFunctionBase
}

func NewLower(operand Expression) Function {
	return &Lower{
		*NewUnaryFunctionBase("lower", operand),
	}
}

func (this *Lower) Accept(visitor Visitor) (interface{}, error) {
	return visitor.VisitFunction(this)
}

func (this *Lower) Type() value.Type { return value.STRING }

func (this *Lower) Evaluate(item value.Value, context Context) (value.Value, error) {
	return this.UnaryEval(this, item, context)
}

func (this *Lower) Apply(context Context, arg value.Value) (value.Value, error) {
	if arg.Type() == value.MISSING {
		return value.MISSING_VALUE, nil
	} else if arg.Type() != value.STRING {
		return value.NULL_VALUE, nil
	}

	rv := strings.ToLower(arg.Actual().(string))
	return value.NewValue(rv), nil
}

func (this *Lower) Constructor() FunctionConstructor {
	return func(operands ...Expression) Function {
		return NewLower(operands[0])
	}
}

///////////////////////////////////////////////////
//
// LTrim
//
///////////////////////////////////////////////////

type LTrim struct {
	FunctionBase
}

func NewLTrim(operands ...Expression) Function {
	return &LTrim{
		*NewFunctionBase("ltrim", operands...),
	}
}

func (this *LTrim) Accept(visitor Visitor) (interface{}, error) {
	return visitor.VisitFunction(this)
}

func (this *LTrim) Type() value.Type { return value.STRING }

func (this *LTrim) Evaluate(item value.Value, context Context) (value.Value, error) {
	return this.Eval(this, item, context)
}

func (this *LTrim) Apply(context Context, args ...value.Value) (value.Value, error) {
	null := false

	for _, a := range args {
		if a.Type() == value.MISSING {
			return value.MISSING_VALUE, nil
		} else if a.Type() != value.STRING {
			null = true
		}
	}

	if null {
		return value.NULL_VALUE, nil
	}

	chars := _WHITESPACE
	if len(args) > 1 {
		chars = args[1]
	}

	rv := strings.TrimLeft(args[0].Actual().(string), chars.Actual().(string))
	return value.NewValue(rv), nil
}

func (this *LTrim) MinArgs() int { return 1 }

func (this *LTrim) MaxArgs() int { return 2 }

func (this *LTrim) Constructor() FunctionConstructor { return NewLTrim }

var _WHITESPACE = value.NewValue(" \t\n\f\r")

///////////////////////////////////////////////////
//
// Position
//
///////////////////////////////////////////////////

type Position struct {
	BinaryFunctionBase
}

func NewPosition(first, second Expression) Function {
	return &Position{
		*NewBinaryFunctionBase("position", first, second),
	}
}

func (this *Position) Accept(visitor Visitor) (interface{}, error) {
	return visitor.VisitFunction(this)
}

func (this *Position) Type() value.Type { return value.NUMBER }

func (this *Position) Evaluate(item value.Value, context Context) (value.Value, error) {
	return this.BinaryEval(this, item, context)
}

func (this *Position) Apply(context Context, first, second value.Value) (value.Value, error) {
	if first.Type() == value.MISSING || second.Type() == value.MISSING {
		return value.MISSING_VALUE, nil
	} else if first.Type() != value.STRING || second.Type() != value.STRING {
		return value.NULL_VALUE, nil
	}

	rv := strings.Index(first.Actual().(string), second.Actual().(string))
	return value.NewValue(float64(rv)), nil
}

func (this *Position) Constructor() FunctionConstructor {
	return func(operands ...Expression) Function {
		return NewPosition(operands[0], operands[1])
	}
}

///////////////////////////////////////////////////
//
// Repeat
//
///////////////////////////////////////////////////

type Repeat struct {
	BinaryFunctionBase
}

func NewRepeat(first, second Expression) Function {
	return &Repeat{
		*NewBinaryFunctionBase("repeat", first, second),
	}
}

func (this *Repeat) Accept(visitor Visitor) (interface{}, error) {
	return visitor.VisitFunction(this)
}

func (this *Repeat) Type() value.Type { return value.STRING }

func (this *Repeat) Evaluate(item value.Value, context Context) (value.Value, error) {
	return this.BinaryEval(this, item, context)
}

func (this *Repeat) Apply(context Context, first, second value.Value) (value.Value, error) {
	if first.Type() == value.MISSING || second.Type() == value.MISSING {
		return value.MISSING_VALUE, nil
	} else if first.Type() != value.STRING || second.Type() != value.NUMBER {
		return value.NULL_VALUE, nil
	}

	nf := second.Actual().(float64)
	if nf < 0.0 || nf != math.Trunc(nf) {
		return value.NULL_VALUE, nil
	}

	rv := strings.Repeat(first.Actual().(string), int(nf))
	return value.NewValue(rv), nil
}

func (this *Repeat) Constructor() FunctionConstructor {
	return func(operands ...Expression) Function {
		return NewRepeat(operands[0], operands[1])
	}
}

///////////////////////////////////////////////////
//
// Replace
//
///////////////////////////////////////////////////

type Replace struct {
	FunctionBase
}

func NewReplace(operands ...Expression) Function {
	return &Replace{
		*NewFunctionBase("replace", operands...),
	}
}

func (this *Replace) Accept(visitor Visitor) (interface{}, error) {
	return visitor.VisitFunction(this)
}

func (this *Replace) Type() value.Type { return value.STRING }

func (this *Replace) Evaluate(item value.Value, context Context) (value.Value, error) {
	return this.Eval(this, item, context)
}

func (this *Replace) Apply(context Context, args ...value.Value) (value.Value, error) {
	null := false

	for i := 0; i < 3; i++ {
		if args[i].Type() == value.MISSING {
			return value.MISSING_VALUE, nil
		} else if args[i].Type() != value.STRING {
			null = true
		}
	}

	if null {
		return value.NULL_VALUE, nil
	}

	if len(args) == 4 && args[3].Type() != value.NUMBER {
		return value.NULL_VALUE, nil
	}

	f := args[0].Actual().(string)
	s := args[1].Actual().(string)
	r := args[2].Actual().(string)
	n := -1

	if len(args) == 4 {
		nf := args[3].Actual().(float64)
		if nf != math.Trunc(nf) {
			return value.NULL_VALUE, nil
		}

		n = int(nf)
	}

	rv := strings.Replace(f, s, r, n)
	return value.NewValue(rv), nil
}

func (this *Replace) MinArgs() int { return 3 }

func (this *Replace) MaxArgs() int { return 4 }

func (this *Replace) Constructor() FunctionConstructor { return NewReplace }

///////////////////////////////////////////////////
//
// RTrim
//
///////////////////////////////////////////////////

type RTrim struct {
	FunctionBase
}

func NewRTrim(operands ...Expression) Function {
	return &RTrim{
		*NewFunctionBase("rtrim", operands...),
	}
}

func (this *RTrim) Accept(visitor Visitor) (interface{}, error) {
	return visitor.VisitFunction(this)
}

func (this *RTrim) Type() value.Type { return value.STRING }

func (this *RTrim) Evaluate(item value.Value, context Context) (value.Value, error) {
	return this.Eval(this, item, context)
}

func (this *RTrim) Apply(context Context, args ...value.Value) (value.Value, error) {
	null := false

	for _, a := range args {
		if a.Type() == value.MISSING {
			return value.MISSING_VALUE, nil
		} else if a.Type() != value.STRING {
			null = true
		}
	}

	if null {
		return value.NULL_VALUE, nil
	}

	chars := _WHITESPACE
	if len(args) > 1 {
		chars = args[1]
	}

	rv := strings.TrimRight(args[0].Actual().(string), chars.Actual().(string))
	return value.NewValue(rv), nil
}

func (this *RTrim) MinArgs() int { return 1 }

func (this *RTrim) MaxArgs() int { return 2 }

func (this *RTrim) Constructor() FunctionConstructor { return NewRTrim }

///////////////////////////////////////////////////
//
// Split
//
///////////////////////////////////////////////////

type Split struct {
	FunctionBase
}

func NewSplit(operands ...Expression) Function {
	return &Split{
		*NewFunctionBase("split", operands...),
	}
}

func (this *Split) Accept(visitor Visitor) (interface{}, error) {
	return visitor.VisitFunction(this)
}

func (this *Split) Type() value.Type { return value.STRING }

func (this *Split) Evaluate(item value.Value, context Context) (value.Value, error) {
	return this.Eval(this, item, context)
}

func (this *Split) Apply(context Context, args ...value.Value) (value.Value, error) {
	null := false

	for _, a := range args {
		if a.Type() == value.MISSING {
			return value.MISSING_VALUE, nil
		} else if a.Type() != value.STRING {
			null = true
		}
	}

	if null {
		return value.NULL_VALUE, nil
	}

	var sa []string
	if len(args) > 1 {
		sep := args[1]
		sa = strings.Split(args[0].Actual().(string),
			sep.Actual().(string))
	} else {
		sa = strings.Fields(args[0].Actual().(string))
	}

	rv := make([]interface{}, len(sa))
	for i, s := range sa {
		rv[i] = s
	}

	return value.NewValue(rv), nil
}

func (this *Split) MinArgs() int { return 1 }

func (this *Split) MaxArgs() int { return 2 }

func (this *Split) Constructor() FunctionConstructor { return NewSplit }

///////////////////////////////////////////////////
//
// Substr
//
///////////////////////////////////////////////////

type Substr struct {
	FunctionBase
}

func NewSubstr(operands ...Expression) Function {
	return &Substr{
		*NewFunctionBase("substr", operands...),
	}
}

func (this *Substr) Accept(visitor Visitor) (interface{}, error) {
	return visitor.VisitFunction(this)
}

func (this *Substr) Type() value.Type { return value.STRING }

func (this *Substr) Evaluate(item value.Value, context Context) (value.Value, error) {
	return this.Eval(this, item, context)
}

func (this *Substr) Apply(context Context, args ...value.Value) (value.Value, error) {
	null := false

	if args[0].Type() == value.MISSING {
		return value.MISSING_VALUE, nil
	} else if args[0].Type() != value.STRING {
		null = true
	}

	for i := 1; i < len(args); i++ {
		switch args[i].Type() {
		case value.MISSING:
			return value.MISSING_VALUE, nil
		case value.NUMBER:
			vf := args[i].Actual().(float64)
			if vf != math.Trunc(vf) {
				null = true
			}
		default:
			null = true
		}
	}

	if null {
		return value.NULL_VALUE, nil
	}

	str := args[0].Actual().(string)
	pos := int(args[1].Actual().(float64))

	if pos < 0 {
		pos = len(str) + pos
	}

	if pos < 0 || pos >= len(str) {
		return value.NULL_VALUE, nil
	}

	if len(args) == 2 {
		return value.NewValue(str[pos:]), nil
	}

	length := int(args[2].Actual().(float64))
	if length < 0 || pos+length > len(str) {
		return value.NULL_VALUE, nil
	}

	return value.NewValue(str[pos : pos+length]), nil
}

func (this *Substr) MinArgs() int { return 2 }

func (this *Substr) MaxArgs() int { return 3 }

func (this *Substr) Constructor() FunctionConstructor { return NewSubstr }

///////////////////////////////////////////////////
//
// Title
//
///////////////////////////////////////////////////

type Title struct {
	UnaryFunctionBase
}

func NewTitle(operand Expression) Function {
	return &Title{
		*NewUnaryFunctionBase("title", operand),
	}
}

func (this *Title) Accept(visitor Visitor) (interface{}, error) {
	return visitor.VisitFunction(this)
}

func (this *Title) Type() value.Type { return value.STRING }

func (this *Title) Evaluate(item value.Value, context Context) (value.Value, error) {
	return this.UnaryEval(this, item, context)
}

func (this *Title) Apply(context Context, arg value.Value) (value.Value, error) {
	if arg.Type() == value.MISSING {
		return value.MISSING_VALUE, nil
	} else if arg.Type() != value.STRING {
		return value.NULL_VALUE, nil
	}

	rv := strings.Title(arg.Actual().(string))
	return value.NewValue(rv), nil
}

func (this *Title) Constructor() FunctionConstructor {
	return func(operands ...Expression) Function {
		return NewTitle(operands[0])
	}
}

///////////////////////////////////////////////////
//
// Trim
//
///////////////////////////////////////////////////

type Trim struct {
	FunctionBase
}

func NewTrim(operands ...Expression) Function {
	return &Trim{
		*NewFunctionBase("trim", operands...),
	}
}

func (this *Trim) Accept(visitor Visitor) (interface{}, error) {
	return visitor.VisitFunction(this)
}

func (this *Trim) Type() value.Type { return value.STRING }

func (this *Trim) Evaluate(item value.Value, context Context) (value.Value, error) {
	return this.Eval(this, item, context)
}

func (this *Trim) Apply(context Context, args ...value.Value) (value.Value, error) {
	null := false

	for _, a := range args {
		if a.Type() == value.MISSING {
			return value.MISSING_VALUE, nil
		} else if a.Type() != value.STRING {
			null = true
		}
	}

	if null {
		return value.NULL_VALUE, nil
	}

	chars := _WHITESPACE
	if len(args) > 1 {
		chars = args[1]
	}

	rv := strings.Trim(args[0].Actual().(string), chars.Actual().(string))
	return value.NewValue(rv), nil
}

func (this *Trim) MinArgs() int { return 1 }

func (this *Trim) MaxArgs() int { return 2 }

func (this *Trim) Constructor() FunctionConstructor { return NewTrim }

///////////////////////////////////////////////////
//
// Upper
//
///////////////////////////////////////////////////

type Upper struct {
	UnaryFunctionBase
}

func NewUpper(operand Expression) Function {
	return &Upper{
		*NewUnaryFunctionBase("upper", operand),
	}
}

func (this *Upper) Accept(visitor Visitor) (interface{}, error) {
	return visitor.VisitFunction(this)
}

func (this *Upper) Type() value.Type { return value.STRING }

func (this *Upper) Evaluate(item value.Value, context Context) (value.Value, error) {
	return this.UnaryEval(this, item, context)
}

func (this *Upper) Apply(context Context, arg value.Value) (value.Value, error) {
	if arg.Type() == value.MISSING {
		return value.MISSING_VALUE, nil
	} else if arg.Type() != value.STRING {
		return value.NULL_VALUE, nil
	}

	rv := strings.ToUpper(arg.Actual().(string))
	return value.NewValue(rv), nil
}

func (this *Upper) Constructor() FunctionConstructor {
	return func(operands ...Expression) Function {
		return NewUpper(operands[0])
	}
}
