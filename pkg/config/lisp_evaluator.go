// Package config provides Lisp evaluation with built-in functions
package config

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

// EvalContext holds the context for evaluating Lisp expressions
type EvalContext struct {
	// Variables holds user-defined variables
	Variables map[string]interface{}
	// Functions holds custom function implementations
	Functions map[string]LispFunc
	// WorkDir is the working directory for file operations
	WorkDir string
	// EnvPrefix filters environment variables (optional)
	EnvPrefix string
	// StrictMode enables strict validation during evaluation
	StrictMode bool
	// Errors collects non-fatal errors during evaluation
	Errors []EvalError
}

// EvalError represents an evaluation error
type EvalError struct {
	Expression string
	Message    string
	Line       int
	Column     int
}

func (e EvalError) Error() string {
	if e.Line > 0 {
		return fmt.Sprintf("line %d: %s - %s", e.Line, e.Expression, e.Message)
	}
	return fmt.Sprintf("%s - %s", e.Expression, e.Message)
}

// LispFunc is a function that can be called from Lisp
type LispFunc func(ctx *EvalContext, args []SExpr) (SExpr, error)

// NewEvalContext creates a new evaluation context with default functions
func NewEvalContext() *EvalContext {
	ctx := &EvalContext{
		Variables: make(map[string]interface{}),
		Functions: make(map[string]LispFunc),
		WorkDir:   ".",
		Errors:    make([]EvalError, 0),
	}
	ctx.registerBuiltinFunctions()
	return ctx
}

// registerBuiltinFunctions registers all built-in Lisp functions
func (ctx *EvalContext) registerBuiltinFunctions() {
	// Environment functions
	ctx.Functions["env"] = builtinEnv
	ctx.Functions["getenv"] = builtinEnv
	ctx.Functions["env-or"] = builtinEnvOr
	ctx.Functions["env?"] = builtinEnvExists

	// String functions
	ctx.Functions["concat"] = builtinConcat
	ctx.Functions["str"] = builtinConcat
	ctx.Functions["format"] = builtinFormat
	ctx.Functions["upper"] = builtinUpper
	ctx.Functions["lower"] = builtinLower
	ctx.Functions["trim"] = builtinTrim
	ctx.Functions["split"] = builtinSplit
	ctx.Functions["join"] = builtinJoin
	ctx.Functions["replace"] = builtinReplace
	ctx.Functions["substring"] = builtinSubstring

	// Control flow
	ctx.Functions["if"] = builtinIf
	ctx.Functions["when"] = builtinWhen
	ctx.Functions["unless"] = builtinUnless
	ctx.Functions["cond"] = builtinCond
	ctx.Functions["default"] = builtinDefault
	ctx.Functions["or"] = builtinOr
	ctx.Functions["and"] = builtinAnd
	ctx.Functions["not"] = builtinNot

	// Comparison
	ctx.Functions["eq"] = builtinEq
	ctx.Functions["="] = builtinEq
	ctx.Functions["!="] = builtinNeq
	ctx.Functions["<"] = builtinLt
	ctx.Functions[">"] = builtinGt
	ctx.Functions["<="] = builtinLte
	ctx.Functions[">="] = builtinGte

	// Arithmetic
	ctx.Functions["+"] = builtinAdd
	ctx.Functions["-"] = builtinSub
	ctx.Functions["*"] = builtinMul
	ctx.Functions["/"] = builtinDiv
	ctx.Functions["mod"] = builtinMod

	// Encoding/Hashing
	ctx.Functions["base64-encode"] = builtinBase64Encode
	ctx.Functions["base64-decode"] = builtinBase64Decode
	ctx.Functions["sha256"] = builtinSHA256
	ctx.Functions["md5"] = builtinMD5

	// UUID/Random
	ctx.Functions["uuid"] = builtinUUID
	ctx.Functions["random-string"] = builtinRandomString

	// Time/Date
	ctx.Functions["now"] = builtinNow
	ctx.Functions["timestamp"] = builtinTimestamp
	ctx.Functions["date"] = builtinDate
	ctx.Functions["time"] = builtinTime

	// System
	ctx.Functions["hostname"] = builtinHostname
	ctx.Functions["user"] = builtinUser
	ctx.Functions["home"] = builtinHome
	ctx.Functions["cwd"] = builtinCwd

	// File operations
	ctx.Functions["read-file"] = builtinReadFile
	ctx.Functions["file-exists?"] = builtinFileExists
	ctx.Functions["dirname"] = builtinDirname
	ctx.Functions["basename"] = builtinBasename
	ctx.Functions["expand-path"] = builtinExpandPath

	// Shell execution (with safety checks)
	ctx.Functions["shell"] = builtinShell

	// Variables
	ctx.Functions["let"] = builtinLet
	ctx.Functions["var"] = builtinVar
	ctx.Functions["set"] = builtinSet

	// List operations
	ctx.Functions["list"] = builtinList
	ctx.Functions["first"] = builtinFirst
	ctx.Functions["rest"] = builtinRest
	ctx.Functions["nth"] = builtinNth
	ctx.Functions["len"] = builtinLen
	ctx.Functions["append"] = builtinAppend
	ctx.Functions["range"] = builtinRange

	// Type checking
	ctx.Functions["string?"] = builtinIsString
	ctx.Functions["number?"] = builtinIsNumber
	ctx.Functions["bool?"] = builtinIsBool
	ctx.Functions["list?"] = builtinIsList
	ctx.Functions["nil?"] = builtinIsNil
	ctx.Functions["empty?"] = builtinIsEmpty

	// Type conversion
	ctx.Functions["to-string"] = builtinToString
	ctx.Functions["to-int"] = builtinToInt
	ctx.Functions["to-bool"] = builtinToBool

	// Regex
	ctx.Functions["match"] = builtinMatch
	ctx.Functions["match?"] = builtinMatchP
}

// Eval evaluates an S-expression and returns the result
func (ctx *EvalContext) Eval(expr SExpr) (SExpr, error) {
	if expr == nil {
		return &Atom{Value: nil, RawValue: "nil"}, nil
	}

	switch e := expr.(type) {
	case *Atom:
		// Check if it's a variable reference
		if e.IsSymbol() && !e.IsBool() {
			name := e.AsString()
			if val, ok := ctx.Variables[name]; ok {
				return valueToSExpr(val), nil
			}
		}
		return e, nil

	case *List:
		if len(e.Items) == 0 {
			return e, nil
		}

		// Get the function name
		head := e.Head()
		if head == nil {
			return e, nil
		}

		funcName := head.AsString()

		// Check if it's a built-in function
		if fn, ok := ctx.Functions[funcName]; ok {
			return fn(ctx, e.Tail())
		}

		// Not a function call, evaluate all items and return as list
		return ctx.evalList(e)
	}

	return expr, nil
}

// evalList evaluates all items in a list
func (ctx *EvalContext) evalList(l *List) (*List, error) {
	result := &List{Items: make([]SExpr, len(l.Items))}
	for i, item := range l.Items {
		evaluated, err := ctx.Eval(item)
		if err != nil {
			return nil, err
		}
		result.Items[i] = evaluated
	}
	return result, nil
}

// EvalAll evaluates a root expression and all nested function calls
func (ctx *EvalContext) EvalAll(expr SExpr) (SExpr, error) {
	return ctx.evalRecursive(expr)
}

// evalRecursive recursively evaluates all function calls in an expression
func (ctx *EvalContext) evalRecursive(expr SExpr) (SExpr, error) {
	if expr == nil {
		return nil, nil
	}

	switch e := expr.(type) {
	case *Atom:
		// Check for variable reference
		if e.IsSymbol() && !e.IsBool() {
			name := e.AsString()
			if val, ok := ctx.Variables[name]; ok {
				return valueToSExpr(val), nil
			}
		}
		return e, nil

	case *List:
		if len(e.Items) == 0 {
			return e, nil
		}

		head := e.Head()
		if head == nil {
			return e, nil
		}

		funcName := head.AsString()

		// Check if it's a function call
		if fn, ok := ctx.Functions[funcName]; ok {
			// Evaluate arguments first (except for special forms like 'if', 'let')
			if !isSpecialForm(funcName) {
				evaluatedArgs := make([]SExpr, 0, len(e.Tail()))
				for _, arg := range e.Tail() {
					evaluated, err := ctx.evalRecursive(arg)
					if err != nil {
						return nil, err
					}
					evaluatedArgs = append(evaluatedArgs, evaluated)
				}
				return fn(ctx, evaluatedArgs)
			}
			return fn(ctx, e.Tail())
		}

		// Not a function call, recursively evaluate all items
		result := &List{Items: make([]SExpr, len(e.Items))}
		for i, item := range e.Items {
			evaluated, err := ctx.evalRecursive(item)
			if err != nil {
				return nil, err
			}
			result.Items[i] = evaluated
		}
		return result, nil
	}

	return expr, nil
}

// isSpecialForm returns true for forms that handle their own argument evaluation
func isSpecialForm(name string) bool {
	specialForms := map[string]bool{
		"if":     true,
		"when":   true,
		"unless": true,
		"cond":   true,
		"and":    true,
		"or":     true,
		"let":    true,
	}
	return specialForms[name]
}

// valueToSExpr converts a Go value to an S-expression
func valueToSExpr(v interface{}) SExpr {
	if v == nil {
		return &Atom{Value: nil, RawValue: "nil"}
	}
	switch val := v.(type) {
	case string:
		return &Atom{Value: val, RawValue: fmt.Sprintf("\"%s\"", val)}
	case int, int64, float64:
		return &Atom{Value: val, RawValue: fmt.Sprintf("%v", val)}
	case bool:
		return &Atom{Value: val, RawValue: fmt.Sprintf("%v", val)}
	case []string:
		items := make([]SExpr, len(val))
		for i, s := range val {
			items[i] = &Atom{Value: s, RawValue: fmt.Sprintf("\"%s\"", s)}
		}
		return &List{Items: items}
	case SExpr:
		return val
	default:
		return &Atom{Value: fmt.Sprintf("%v", val), RawValue: fmt.Sprintf("%v", val)}
	}
}

// Helper to get string from SExpr
func sexprToString(expr SExpr) string {
	if expr == nil {
		return ""
	}
	if atom, ok := expr.(*Atom); ok {
		return atom.AsString()
	}
	return expr.String()
}

// Helper to get int from SExpr
func sexprToInt(expr SExpr) int {
	if expr == nil {
		return 0
	}
	if atom, ok := expr.(*Atom); ok {
		return atom.AsInt()
	}
	return 0
}

// Helper to get bool from SExpr
func sexprToBool(expr SExpr) bool {
	if expr == nil {
		return false
	}
	if atom, ok := expr.(*Atom); ok {
		return atom.AsBool()
	}
	// Non-empty list is truthy
	if list, ok := expr.(*List); ok {
		return len(list.Items) > 0
	}
	return false
}

// =============================================================================
// Built-in Functions Implementation
// =============================================================================

// Environment functions

func builtinEnv(ctx *EvalContext, args []SExpr) (SExpr, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("env requires at least 1 argument")
	}
	varName := sexprToString(args[0])
	value := os.Getenv(varName)

	// If env var is empty and default provided
	if value == "" && len(args) >= 2 {
		return args[1], nil
	}

	return &Atom{Value: value, RawValue: fmt.Sprintf("\"%s\"", value)}, nil
}

func builtinEnvOr(ctx *EvalContext, args []SExpr) (SExpr, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("env-or requires 2 arguments: (env-or VAR_NAME default)")
	}
	varName := sexprToString(args[0])
	value := os.Getenv(varName)
	if value == "" {
		return args[1], nil
	}
	return &Atom{Value: value, RawValue: fmt.Sprintf("\"%s\"", value)}, nil
}

func builtinEnvExists(ctx *EvalContext, args []SExpr) (SExpr, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("env? requires 1 argument")
	}
	varName := sexprToString(args[0])
	_, exists := os.LookupEnv(varName)
	return &Atom{Value: exists, RawValue: fmt.Sprintf("%v", exists)}, nil
}

// String functions

func builtinConcat(ctx *EvalContext, args []SExpr) (SExpr, error) {
	var result strings.Builder
	for _, arg := range args {
		result.WriteString(sexprToString(arg))
	}
	return &Atom{Value: result.String(), RawValue: fmt.Sprintf("\"%s\"", result.String())}, nil
}

func builtinFormat(ctx *EvalContext, args []SExpr) (SExpr, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("format requires at least 1 argument")
	}
	template := sexprToString(args[0])

	// Convert remaining args to interface{} for fmt.Sprintf
	fmtArgs := make([]interface{}, len(args)-1)
	for i, arg := range args[1:] {
		fmtArgs[i] = sexprToString(arg)
	}

	result := fmt.Sprintf(template, fmtArgs...)
	return &Atom{Value: result, RawValue: fmt.Sprintf("\"%s\"", result)}, nil
}

func builtinUpper(ctx *EvalContext, args []SExpr) (SExpr, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("upper requires 1 argument")
	}
	result := strings.ToUpper(sexprToString(args[0]))
	return &Atom{Value: result, RawValue: fmt.Sprintf("\"%s\"", result)}, nil
}

func builtinLower(ctx *EvalContext, args []SExpr) (SExpr, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("lower requires 1 argument")
	}
	result := strings.ToLower(sexprToString(args[0]))
	return &Atom{Value: result, RawValue: fmt.Sprintf("\"%s\"", result)}, nil
}

func builtinTrim(ctx *EvalContext, args []SExpr) (SExpr, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("trim requires 1 argument")
	}
	result := strings.TrimSpace(sexprToString(args[0]))
	return &Atom{Value: result, RawValue: fmt.Sprintf("\"%s\"", result)}, nil
}

func builtinSplit(ctx *EvalContext, args []SExpr) (SExpr, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("split requires 2 arguments: (split string separator)")
	}
	str := sexprToString(args[0])
	sep := sexprToString(args[1])
	parts := strings.Split(str, sep)

	items := make([]SExpr, len(parts))
	for i, p := range parts {
		items[i] = &Atom{Value: p, RawValue: fmt.Sprintf("\"%s\"", p)}
	}
	return &List{Items: items}, nil
}

func builtinJoin(ctx *EvalContext, args []SExpr) (SExpr, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("join requires 2 arguments: (join list separator)")
	}

	var parts []string
	if list, ok := args[0].(*List); ok {
		for _, item := range list.Items {
			parts = append(parts, sexprToString(item))
		}
	}

	sep := sexprToString(args[1])
	result := strings.Join(parts, sep)
	return &Atom{Value: result, RawValue: fmt.Sprintf("\"%s\"", result)}, nil
}

func builtinReplace(ctx *EvalContext, args []SExpr) (SExpr, error) {
	if len(args) < 3 {
		return nil, fmt.Errorf("replace requires 3 arguments: (replace string old new)")
	}
	str := sexprToString(args[0])
	old := sexprToString(args[1])
	newStr := sexprToString(args[2])
	result := strings.ReplaceAll(str, old, newStr)
	return &Atom{Value: result, RawValue: fmt.Sprintf("\"%s\"", result)}, nil
}

func builtinSubstring(ctx *EvalContext, args []SExpr) (SExpr, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("substring requires at least 2 arguments: (substring string start [end])")
	}
	str := sexprToString(args[0])
	start := sexprToInt(args[1])

	if start < 0 || start > len(str) {
		start = 0
	}

	end := len(str)
	if len(args) >= 3 {
		end = sexprToInt(args[2])
		if end > len(str) {
			end = len(str)
		}
	}

	result := str[start:end]
	return &Atom{Value: result, RawValue: fmt.Sprintf("\"%s\"", result)}, nil
}

// Control flow

func builtinIf(ctx *EvalContext, args []SExpr) (SExpr, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("if requires at least 2 arguments: (if condition then [else])")
	}

	// Evaluate condition
	condResult, err := ctx.evalRecursive(args[0])
	if err != nil {
		return nil, err
	}

	if sexprToBool(condResult) {
		return ctx.evalRecursive(args[1])
	} else if len(args) >= 3 {
		return ctx.evalRecursive(args[2])
	}
	return &Atom{Value: nil, RawValue: "nil"}, nil
}

func builtinWhen(ctx *EvalContext, args []SExpr) (SExpr, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("when requires at least 2 arguments")
	}

	condResult, err := ctx.evalRecursive(args[0])
	if err != nil {
		return nil, err
	}

	if sexprToBool(condResult) {
		var result SExpr
		for _, arg := range args[1:] {
			result, err = ctx.evalRecursive(arg)
			if err != nil {
				return nil, err
			}
		}
		return result, nil
	}
	return &Atom{Value: nil, RawValue: "nil"}, nil
}

func builtinUnless(ctx *EvalContext, args []SExpr) (SExpr, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("unless requires at least 2 arguments")
	}

	condResult, err := ctx.evalRecursive(args[0])
	if err != nil {
		return nil, err
	}

	if !sexprToBool(condResult) {
		var result SExpr
		for _, arg := range args[1:] {
			result, err = ctx.evalRecursive(arg)
			if err != nil {
				return nil, err
			}
		}
		return result, nil
	}
	return &Atom{Value: nil, RawValue: "nil"}, nil
}

func builtinCond(ctx *EvalContext, args []SExpr) (SExpr, error) {
	for _, arg := range args {
		if clause, ok := arg.(*List); ok && len(clause.Items) >= 2 {
			condResult, err := ctx.evalRecursive(clause.Items[0])
			if err != nil {
				return nil, err
			}
			if sexprToBool(condResult) {
				return ctx.evalRecursive(clause.Items[1])
			}
		}
	}
	return &Atom{Value: nil, RawValue: "nil"}, nil
}

func builtinDefault(ctx *EvalContext, args []SExpr) (SExpr, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("default requires 2 arguments: (default value fallback)")
	}

	value := args[0]
	if value == nil {
		return args[1], nil
	}
	if atom, ok := value.(*Atom); ok {
		if atom.Value == nil || atom.AsString() == "" {
			return args[1], nil
		}
	}
	return value, nil
}

func builtinOr(ctx *EvalContext, args []SExpr) (SExpr, error) {
	for _, arg := range args {
		result, err := ctx.evalRecursive(arg)
		if err != nil {
			return nil, err
		}
		if sexprToBool(result) {
			return result, nil
		}
	}
	return &Atom{Value: false, RawValue: "false"}, nil
}

func builtinAnd(ctx *EvalContext, args []SExpr) (SExpr, error) {
	var result SExpr = &Atom{Value: true, RawValue: "true"}
	for _, arg := range args {
		var err error
		result, err = ctx.evalRecursive(arg)
		if err != nil {
			return nil, err
		}
		if !sexprToBool(result) {
			return &Atom{Value: false, RawValue: "false"}, nil
		}
	}
	return result, nil
}

func builtinNot(ctx *EvalContext, args []SExpr) (SExpr, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("not requires 1 argument")
	}
	result := !sexprToBool(args[0])
	return &Atom{Value: result, RawValue: fmt.Sprintf("%v", result)}, nil
}

// Comparison functions

func builtinEq(ctx *EvalContext, args []SExpr) (SExpr, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("eq requires 2 arguments")
	}
	result := sexprToString(args[0]) == sexprToString(args[1])
	return &Atom{Value: result, RawValue: fmt.Sprintf("%v", result)}, nil
}

func builtinNeq(ctx *EvalContext, args []SExpr) (SExpr, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("!= requires 2 arguments")
	}
	result := sexprToString(args[0]) != sexprToString(args[1])
	return &Atom{Value: result, RawValue: fmt.Sprintf("%v", result)}, nil
}

func builtinLt(ctx *EvalContext, args []SExpr) (SExpr, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("< requires 2 arguments")
	}
	result := sexprToInt(args[0]) < sexprToInt(args[1])
	return &Atom{Value: result, RawValue: fmt.Sprintf("%v", result)}, nil
}

func builtinGt(ctx *EvalContext, args []SExpr) (SExpr, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("> requires 2 arguments")
	}
	result := sexprToInt(args[0]) > sexprToInt(args[1])
	return &Atom{Value: result, RawValue: fmt.Sprintf("%v", result)}, nil
}

func builtinLte(ctx *EvalContext, args []SExpr) (SExpr, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("<= requires 2 arguments")
	}
	result := sexprToInt(args[0]) <= sexprToInt(args[1])
	return &Atom{Value: result, RawValue: fmt.Sprintf("%v", result)}, nil
}

func builtinGte(ctx *EvalContext, args []SExpr) (SExpr, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf(">= requires 2 arguments")
	}
	result := sexprToInt(args[0]) >= sexprToInt(args[1])
	return &Atom{Value: result, RawValue: fmt.Sprintf("%v", result)}, nil
}

// Arithmetic functions

func builtinAdd(ctx *EvalContext, args []SExpr) (SExpr, error) {
	result := 0
	for _, arg := range args {
		result += sexprToInt(arg)
	}
	return &Atom{Value: result, RawValue: fmt.Sprintf("%d", result)}, nil
}

func builtinSub(ctx *EvalContext, args []SExpr) (SExpr, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("- requires at least 1 argument")
	}
	result := sexprToInt(args[0])
	for _, arg := range args[1:] {
		result -= sexprToInt(arg)
	}
	return &Atom{Value: result, RawValue: fmt.Sprintf("%d", result)}, nil
}

func builtinMul(ctx *EvalContext, args []SExpr) (SExpr, error) {
	result := 1
	for _, arg := range args {
		result *= sexprToInt(arg)
	}
	return &Atom{Value: result, RawValue: fmt.Sprintf("%d", result)}, nil
}

func builtinDiv(ctx *EvalContext, args []SExpr) (SExpr, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("/ requires 2 arguments")
	}
	divisor := sexprToInt(args[1])
	if divisor == 0 {
		return nil, fmt.Errorf("division by zero")
	}
	result := sexprToInt(args[0]) / divisor
	return &Atom{Value: result, RawValue: fmt.Sprintf("%d", result)}, nil
}

func builtinMod(ctx *EvalContext, args []SExpr) (SExpr, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("mod requires 2 arguments")
	}
	divisor := sexprToInt(args[1])
	if divisor == 0 {
		return nil, fmt.Errorf("division by zero")
	}
	result := sexprToInt(args[0]) % divisor
	return &Atom{Value: result, RawValue: fmt.Sprintf("%d", result)}, nil
}

// Encoding functions

func builtinBase64Encode(ctx *EvalContext, args []SExpr) (SExpr, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("base64-encode requires 1 argument")
	}
	result := base64.StdEncoding.EncodeToString([]byte(sexprToString(args[0])))
	return &Atom{Value: result, RawValue: fmt.Sprintf("\"%s\"", result)}, nil
}

func builtinBase64Decode(ctx *EvalContext, args []SExpr) (SExpr, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("base64-decode requires 1 argument")
	}
	decoded, err := base64.StdEncoding.DecodeString(sexprToString(args[0]))
	if err != nil {
		return nil, fmt.Errorf("base64-decode failed: %w", err)
	}
	result := string(decoded)
	return &Atom{Value: result, RawValue: fmt.Sprintf("\"%s\"", result)}, nil
}

func builtinSHA256(ctx *EvalContext, args []SExpr) (SExpr, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("sha256 requires 1 argument")
	}
	hash := sha256.Sum256([]byte(sexprToString(args[0])))
	result := hex.EncodeToString(hash[:])
	return &Atom{Value: result, RawValue: fmt.Sprintf("\"%s\"", result)}, nil
}

func builtinMD5(ctx *EvalContext, args []SExpr) (SExpr, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("md5 requires 1 argument")
	}
	// Using SHA256 truncated for simplicity (MD5 is deprecated)
	hash := sha256.Sum256([]byte(sexprToString(args[0])))
	result := hex.EncodeToString(hash[:16])
	return &Atom{Value: result, RawValue: fmt.Sprintf("\"%s\"", result)}, nil
}

// UUID/Random functions

func builtinUUID(ctx *EvalContext, args []SExpr) (SExpr, error) {
	result := uuid.New().String()
	return &Atom{Value: result, RawValue: fmt.Sprintf("\"%s\"", result)}, nil
}

func builtinRandomString(ctx *EvalContext, args []SExpr) (SExpr, error) {
	length := 16
	if len(args) >= 1 {
		length = sexprToInt(args[0])
	}
	if length <= 0 {
		length = 16
	}

	// Generate random string using UUID
	result := strings.ReplaceAll(uuid.New().String(), "-", "")
	if len(result) > length {
		result = result[:length]
	}
	return &Atom{Value: result, RawValue: fmt.Sprintf("\"%s\"", result)}, nil
}

// Time functions

func builtinNow(ctx *EvalContext, args []SExpr) (SExpr, error) {
	result := time.Now().UTC().Format(time.RFC3339)
	return &Atom{Value: result, RawValue: fmt.Sprintf("\"%s\"", result)}, nil
}

func builtinTimestamp(ctx *EvalContext, args []SExpr) (SExpr, error) {
	result := time.Now().Unix()
	return &Atom{Value: int(result), RawValue: fmt.Sprintf("%d", result)}, nil
}

func builtinDate(ctx *EvalContext, args []SExpr) (SExpr, error) {
	format := "2006-01-02"
	if len(args) >= 1 {
		format = sexprToString(args[0])
	}
	result := time.Now().Format(format)
	return &Atom{Value: result, RawValue: fmt.Sprintf("\"%s\"", result)}, nil
}

func builtinTime(ctx *EvalContext, args []SExpr) (SExpr, error) {
	format := "15:04:05"
	if len(args) >= 1 {
		format = sexprToString(args[0])
	}
	result := time.Now().Format(format)
	return &Atom{Value: result, RawValue: fmt.Sprintf("\"%s\"", result)}, nil
}

// System functions

func builtinHostname(ctx *EvalContext, args []SExpr) (SExpr, error) {
	result, err := os.Hostname()
	if err != nil {
		return nil, err
	}
	return &Atom{Value: result, RawValue: fmt.Sprintf("\"%s\"", result)}, nil
}

func builtinUser(ctx *EvalContext, args []SExpr) (SExpr, error) {
	result := os.Getenv("USER")
	if result == "" {
		result = os.Getenv("USERNAME")
	}
	return &Atom{Value: result, RawValue: fmt.Sprintf("\"%s\"", result)}, nil
}

func builtinHome(ctx *EvalContext, args []SExpr) (SExpr, error) {
	result, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	return &Atom{Value: result, RawValue: fmt.Sprintf("\"%s\"", result)}, nil
}

func builtinCwd(ctx *EvalContext, args []SExpr) (SExpr, error) {
	result, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	return &Atom{Value: result, RawValue: fmt.Sprintf("\"%s\"", result)}, nil
}

// File functions

func builtinReadFile(ctx *EvalContext, args []SExpr) (SExpr, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("read-file requires 1 argument")
	}
	path := sexprToString(args[0])

	// Expand ~ to home directory
	if strings.HasPrefix(path, "~") {
		home, _ := os.UserHomeDir()
		path = home + path[1:]
	}

	// Make relative paths relative to WorkDir
	if !filepath.IsAbs(path) {
		path = filepath.Join(ctx.WorkDir, path)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read-file failed: %w", err)
	}

	result := strings.TrimSpace(string(content))
	return &Atom{Value: result, RawValue: fmt.Sprintf("\"%s\"", result)}, nil
}

func builtinFileExists(ctx *EvalContext, args []SExpr) (SExpr, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("file-exists? requires 1 argument")
	}
	path := sexprToString(args[0])

	if strings.HasPrefix(path, "~") {
		home, _ := os.UserHomeDir()
		path = home + path[1:]
	}

	_, err := os.Stat(path)
	result := err == nil
	return &Atom{Value: result, RawValue: fmt.Sprintf("%v", result)}, nil
}

func builtinDirname(ctx *EvalContext, args []SExpr) (SExpr, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("dirname requires 1 argument")
	}
	result := filepath.Dir(sexprToString(args[0]))
	return &Atom{Value: result, RawValue: fmt.Sprintf("\"%s\"", result)}, nil
}

func builtinBasename(ctx *EvalContext, args []SExpr) (SExpr, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("basename requires 1 argument")
	}
	result := filepath.Base(sexprToString(args[0]))
	return &Atom{Value: result, RawValue: fmt.Sprintf("\"%s\"", result)}, nil
}

func builtinExpandPath(ctx *EvalContext, args []SExpr) (SExpr, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("expand-path requires 1 argument")
	}
	path := sexprToString(args[0])

	if strings.HasPrefix(path, "~") {
		home, _ := os.UserHomeDir()
		path = home + path[1:]
	}

	result, _ := filepath.Abs(path)
	return &Atom{Value: result, RawValue: fmt.Sprintf("\"%s\"", result)}, nil
}

// Shell function (with safety checks)

func builtinShell(ctx *EvalContext, args []SExpr) (SExpr, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("shell requires 1 argument")
	}

	command := sexprToString(args[0])

	// Safety check: block dangerous commands
	dangerous := []string{"rm -rf /", "mkfs", "dd if=", ":(){", "fork bomb"}
	for _, d := range dangerous {
		if strings.Contains(command, d) {
			return nil, fmt.Errorf("shell: dangerous command blocked")
		}
	}

	cmd := exec.Command("sh", "-c", command)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("shell command failed: %w", err)
	}

	result := strings.TrimSpace(string(output))
	return &Atom{Value: result, RawValue: fmt.Sprintf("\"%s\"", result)}, nil
}

// Variable functions

func builtinLet(ctx *EvalContext, args []SExpr) (SExpr, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("let requires at least 2 arguments: (let ((var1 val1) (var2 val2)) body)")
	}

	// Save current variables
	savedVars := make(map[string]interface{})
	for k, v := range ctx.Variables {
		savedVars[k] = v
	}

	// Parse bindings
	if bindings, ok := args[0].(*List); ok {
		for _, binding := range bindings.Items {
			if pair, ok := binding.(*List); ok && len(pair.Items) >= 2 {
				if nameAtom, ok := pair.Items[0].(*Atom); ok {
					name := nameAtom.AsString()
					value, err := ctx.evalRecursive(pair.Items[1])
					if err != nil {
						return nil, err
					}
					if atom, ok := value.(*Atom); ok {
						ctx.Variables[name] = atom.Value
					} else {
						ctx.Variables[name] = value
					}
				}
			}
		}
	}

	// Evaluate body
	var result SExpr
	var err error
	for _, bodyExpr := range args[1:] {
		result, err = ctx.evalRecursive(bodyExpr)
		if err != nil {
			break
		}
	}

	// Restore variables
	ctx.Variables = savedVars

	return result, err
}

func builtinVar(ctx *EvalContext, args []SExpr) (SExpr, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("var requires 1 argument")
	}
	name := sexprToString(args[0])
	if val, ok := ctx.Variables[name]; ok {
		return valueToSExpr(val), nil
	}
	return &Atom{Value: nil, RawValue: "nil"}, nil
}

func builtinSet(ctx *EvalContext, args []SExpr) (SExpr, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("set requires 2 arguments: (set name value)")
	}
	name := sexprToString(args[0])
	value := args[1]

	if atom, ok := value.(*Atom); ok {
		ctx.Variables[name] = atom.Value
	} else {
		ctx.Variables[name] = value
	}

	return value, nil
}

// List functions

func builtinList(ctx *EvalContext, args []SExpr) (SExpr, error) {
	return &List{Items: args}, nil
}

func builtinFirst(ctx *EvalContext, args []SExpr) (SExpr, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("first requires 1 argument")
	}
	if list, ok := args[0].(*List); ok && len(list.Items) > 0 {
		return list.Items[0], nil
	}
	return &Atom{Value: nil, RawValue: "nil"}, nil
}

func builtinRest(ctx *EvalContext, args []SExpr) (SExpr, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("rest requires 1 argument")
	}
	if list, ok := args[0].(*List); ok && len(list.Items) > 1 {
		return &List{Items: list.Items[1:]}, nil
	}
	return &List{Items: nil}, nil
}

func builtinNth(ctx *EvalContext, args []SExpr) (SExpr, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("nth requires 2 arguments: (nth list index)")
	}
	if list, ok := args[0].(*List); ok {
		index := sexprToInt(args[1])
		if index >= 0 && index < len(list.Items) {
			return list.Items[index], nil
		}
	}
	return &Atom{Value: nil, RawValue: "nil"}, nil
}

func builtinLen(ctx *EvalContext, args []SExpr) (SExpr, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("len requires 1 argument")
	}
	switch v := args[0].(type) {
	case *List:
		return &Atom{Value: len(v.Items), RawValue: fmt.Sprintf("%d", len(v.Items))}, nil
	case *Atom:
		return &Atom{Value: len(v.AsString()), RawValue: fmt.Sprintf("%d", len(v.AsString()))}, nil
	}
	return &Atom{Value: 0, RawValue: "0"}, nil
}

func builtinAppend(ctx *EvalContext, args []SExpr) (SExpr, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("append requires at least 2 arguments")
	}

	result := &List{Items: make([]SExpr, 0)}
	for _, arg := range args {
		if list, ok := arg.(*List); ok {
			result.Items = append(result.Items, list.Items...)
		} else {
			result.Items = append(result.Items, arg)
		}
	}
	return result, nil
}

func builtinRange(ctx *EvalContext, args []SExpr) (SExpr, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("range requires at least 1 argument")
	}

	start := 0
	end := sexprToInt(args[0])
	step := 1

	if len(args) >= 2 {
		start = end
		end = sexprToInt(args[1])
	}
	if len(args) >= 3 {
		step = sexprToInt(args[2])
	}

	if step == 0 {
		return nil, fmt.Errorf("range: step cannot be zero")
	}

	items := make([]SExpr, 0)
	if step > 0 {
		for i := start; i < end; i += step {
			items = append(items, &Atom{Value: i, RawValue: fmt.Sprintf("%d", i)})
		}
	} else {
		for i := start; i > end; i += step {
			items = append(items, &Atom{Value: i, RawValue: fmt.Sprintf("%d", i)})
		}
	}

	return &List{Items: items}, nil
}

// Type checking functions

func builtinIsString(ctx *EvalContext, args []SExpr) (SExpr, error) {
	if len(args) < 1 {
		return &Atom{Value: false, RawValue: "false"}, nil
	}
	if atom, ok := args[0].(*Atom); ok {
		result := atom.IsString()
		return &Atom{Value: result, RawValue: fmt.Sprintf("%v", result)}, nil
	}
	return &Atom{Value: false, RawValue: "false"}, nil
}

func builtinIsNumber(ctx *EvalContext, args []SExpr) (SExpr, error) {
	if len(args) < 1 {
		return &Atom{Value: false, RawValue: "false"}, nil
	}
	if atom, ok := args[0].(*Atom); ok {
		result := atom.IsNumber()
		return &Atom{Value: result, RawValue: fmt.Sprintf("%v", result)}, nil
	}
	return &Atom{Value: false, RawValue: "false"}, nil
}

func builtinIsBool(ctx *EvalContext, args []SExpr) (SExpr, error) {
	if len(args) < 1 {
		return &Atom{Value: false, RawValue: "false"}, nil
	}
	if atom, ok := args[0].(*Atom); ok {
		result := atom.IsBool()
		return &Atom{Value: result, RawValue: fmt.Sprintf("%v", result)}, nil
	}
	return &Atom{Value: false, RawValue: "false"}, nil
}

func builtinIsList(ctx *EvalContext, args []SExpr) (SExpr, error) {
	if len(args) < 1 {
		return &Atom{Value: false, RawValue: "false"}, nil
	}
	_, ok := args[0].(*List)
	return &Atom{Value: ok, RawValue: fmt.Sprintf("%v", ok)}, nil
}

func builtinIsNil(ctx *EvalContext, args []SExpr) (SExpr, error) {
	if len(args) < 1 {
		return &Atom{Value: true, RawValue: "true"}, nil
	}
	if args[0] == nil {
		return &Atom{Value: true, RawValue: "true"}, nil
	}
	if atom, ok := args[0].(*Atom); ok {
		result := atom.Value == nil
		return &Atom{Value: result, RawValue: fmt.Sprintf("%v", result)}, nil
	}
	return &Atom{Value: false, RawValue: "false"}, nil
}

func builtinIsEmpty(ctx *EvalContext, args []SExpr) (SExpr, error) {
	if len(args) < 1 {
		return &Atom{Value: true, RawValue: "true"}, nil
	}
	switch v := args[0].(type) {
	case *List:
		result := len(v.Items) == 0
		return &Atom{Value: result, RawValue: fmt.Sprintf("%v", result)}, nil
	case *Atom:
		result := v.AsString() == ""
		return &Atom{Value: result, RawValue: fmt.Sprintf("%v", result)}, nil
	}
	return &Atom{Value: true, RawValue: "true"}, nil
}

// Type conversion functions

func builtinToString(ctx *EvalContext, args []SExpr) (SExpr, error) {
	if len(args) < 1 {
		return &Atom{Value: "", RawValue: "\"\""}, nil
	}
	result := sexprToString(args[0])
	return &Atom{Value: result, RawValue: fmt.Sprintf("\"%s\"", result)}, nil
}

func builtinToInt(ctx *EvalContext, args []SExpr) (SExpr, error) {
	if len(args) < 1 {
		return &Atom{Value: 0, RawValue: "0"}, nil
	}
	result := sexprToInt(args[0])
	return &Atom{Value: result, RawValue: fmt.Sprintf("%d", result)}, nil
}

func builtinToBool(ctx *EvalContext, args []SExpr) (SExpr, error) {
	if len(args) < 1 {
		return &Atom{Value: false, RawValue: "false"}, nil
	}
	result := sexprToBool(args[0])
	return &Atom{Value: result, RawValue: fmt.Sprintf("%v", result)}, nil
}

// Regex functions

func builtinMatch(ctx *EvalContext, args []SExpr) (SExpr, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("match requires 2 arguments: (match pattern string)")
	}
	pattern := sexprToString(args[0])
	str := sexprToString(args[1])

	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("match: invalid pattern: %w", err)
	}

	matches := re.FindStringSubmatch(str)
	if matches == nil {
		return &List{Items: nil}, nil
	}

	items := make([]SExpr, len(matches))
	for i, m := range matches {
		items[i] = &Atom{Value: m, RawValue: fmt.Sprintf("\"%s\"", m)}
	}
	return &List{Items: items}, nil
}

func builtinMatchP(ctx *EvalContext, args []SExpr) (SExpr, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("match? requires 2 arguments: (match? pattern string)")
	}
	pattern := sexprToString(args[0])
	str := sexprToString(args[1])

	matched, err := regexp.MatchString(pattern, str)
	if err != nil {
		return nil, fmt.Errorf("match?: invalid pattern: %w", err)
	}

	return &Atom{Value: matched, RawValue: fmt.Sprintf("%v", matched)}, nil
}
