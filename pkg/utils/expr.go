package utils

import (
	"fmt"
	"strings"

	lep "github.com/mgudov/logic-expression-parser"
	"github.com/minio/minio-go/v7/pkg/set"
	"github.com/multiformats/go-multibase"

	"github.com/sirupsen/logrus"
)

var (
	ErrInvalidExpr = fmt.Errorf("invalid expression, expected key=val")
)

func EncodeExpression(in string) (res string, err error) {
	out := []string{}
	for _, ele := range strings.Split(in, " ") {
		if strings.Contains(ele, "=") {
			enc, err := EncodeEle(ele)
			if err != nil {
				return res, err
			}
			out = append(out, enc)
		} else {
			out = append(out, ele)
		}

	}
	return strings.Join(out, " "), nil
}

func EncodeEle(in string) (out string, err error) {
	sl := strings.Split(in, "=")
	if len(sl) != 2 {
		return out, ErrInvalidExpr
	}
	enc, _ := multibase.Encode(multibase.Base16, []byte(sl[0]))
	return fmt.Sprintf("%s=%s", enc, sl[1]), nil
}

func DecodeKey(k string) (res string, err error) {
	e, b, _ := multibase.Decode(k)
	if e != multibase.Base16 {
		err = fmt.Errorf("not base16")
		return
	}
	res = string(b)
	return
}

func ExprGetLabels(q string) (set.StringSet, error) {
	args := set.NewStringSet()
	// TODO: Encode expression
	encQ, err := EncodeExpression(q)
	if err != nil {
		return args, err
	}
	expr, err := lep.ParseExpression(encQ)
	if err != nil {
		return args, fmt.Errorf("failed to parse expression: %s", err.Error())
	}
	return exprGetLabels(args, expr)
}

// ExprGetLabels returns a list of labels that are used in the expression
func exprGetLabels(args set.StringSet, expr lep.Expression) (set.StringSet, error) {
	switch e := expr.(type) {
	default:
		return args, fmt.Errorf("not implemented: %T", e)
	case *lep.OrX:
		for _, disjunction := range e.Disjunctions {
			na, err := exprGetLabels(args, disjunction)
			if err != nil {
				return args, err
			}
			args.Union(na)
		}
		return args, nil
	case *lep.AndX:
		for _, conjunct := range e.Conjuncts {
			na, err := exprGetLabels(args, conjunct)
			if err != nil {
				return args, err
			}
			args.Union(na)
		}
		return args, nil
	case *lep.EqualsX:
		value := e.Value.Value()
		if value == nil {
			return args, nil
		}
		para := e.Param.String()
		switch value.(type) {
		case string:
			logrus.Warnf("Param: '%s' Value: %s", para, value)
		case int:
			logrus.Warnf("Param: '%s' Value: %d", para, value)
		case bool:
			// For bool we need to check is the rule evaluates to true
			rLabel, err := DecodeKey(para)
			if err != nil {
				return args, err
			}
			args.Add(rLabel)
			return args, nil
		}
		return args, nil
	case *lep.NotEqualsX:
		value := e.Value.Value()
		if value == nil {
			return args, nil
		}
		return args, nil
	case *lep.GreaterThanX:
		return args, nil
	}
}
