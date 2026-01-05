package config

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewEvalContext(t *testing.T) {
	ctx := NewEvalContext()
	assert.NotNil(t, ctx)
	assert.NotNil(t, ctx.Variables)
	assert.NotNil(t, ctx.Functions)
	assert.True(t, len(ctx.Functions) > 0, "should have built-in functions")
}

func TestBuiltinEnv(t *testing.T) {
	ctx := NewEvalContext()

	// Set a test env var
	os.Setenv("TEST_LISP_VAR", "test_value")
	defer os.Unsetenv("TEST_LISP_VAR")

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "get existing env var",
			input:    `(env "TEST_LISP_VAR")`,
			expected: "test_value",
		},
		{
			name:     "get non-existing env var with default",
			input:    `(env "NON_EXISTING_VAR" "default_value")`,
			expected: "default_value",
		},
		{
			name:     "get non-existing env var without default",
			input:    `(env "NON_EXISTING_VAR")`,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewLispParser(tt.input)
			expr, err := parser.Parse()
			require.NoError(t, err)

			result, err := ctx.EvalAll(expr)
			require.NoError(t, err)

			if atom, ok := result.(*Atom); ok {
				assert.Equal(t, tt.expected, atom.AsString())
			}
		})
	}
}

func TestBuiltinEnvExists(t *testing.T) {
	ctx := NewEvalContext()

	os.Setenv("TEST_EXISTS_VAR", "value")
	defer os.Unsetenv("TEST_EXISTS_VAR")

	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "existing var returns true",
			input:    `(env? "TEST_EXISTS_VAR")`,
			expected: true,
		},
		{
			name:     "non-existing var returns false",
			input:    `(env? "NON_EXISTING_VAR_12345")`,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewLispParser(tt.input)
			expr, err := parser.Parse()
			require.NoError(t, err)

			result, err := ctx.EvalAll(expr)
			require.NoError(t, err)

			if atom, ok := result.(*Atom); ok {
				assert.Equal(t, tt.expected, atom.AsBool())
			}
		})
	}
}

func TestBuiltinConcat(t *testing.T) {
	ctx := NewEvalContext()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "concat two strings",
			input:    `(concat "hello" "world")`,
			expected: "helloworld",
		},
		{
			name:     "concat three strings",
			input:    `(concat "a" "-" "b")`,
			expected: "a-b",
		},
		{
			name:     "concat with number",
			input:    `(concat "count: " 42)`,
			expected: "count: 42",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewLispParser(tt.input)
			expr, err := parser.Parse()
			require.NoError(t, err)

			result, err := ctx.EvalAll(expr)
			require.NoError(t, err)

			if atom, ok := result.(*Atom); ok {
				assert.Equal(t, tt.expected, atom.AsString())
			}
		})
	}
}

func TestBuiltinFormat(t *testing.T) {
	ctx := NewEvalContext()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "format with one arg",
			input:    `(format "Hello %s" "World")`,
			expected: "Hello World",
		},
		{
			name:     "format with multiple args",
			input:    `(format "%s-%s-%s" "a" "b" "c")`,
			expected: "a-b-c",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewLispParser(tt.input)
			expr, err := parser.Parse()
			require.NoError(t, err)

			result, err := ctx.EvalAll(expr)
			require.NoError(t, err)

			if atom, ok := result.(*Atom); ok {
				assert.Equal(t, tt.expected, atom.AsString())
			}
		})
	}
}

func TestBuiltinStringFunctions(t *testing.T) {
	ctx := NewEvalContext()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "upper",
			input:    `(upper "hello")`,
			expected: "HELLO",
		},
		{
			name:     "lower",
			input:    `(lower "HELLO")`,
			expected: "hello",
		},
		{
			name:     "trim",
			input:    `(trim "  hello  ")`,
			expected: "hello",
		},
		{
			name:     "replace",
			input:    `(replace "hello world" "world" "lisp")`,
			expected: "hello lisp",
		},
		{
			name:     "substring",
			input:    `(substring "hello" 0 2)`,
			expected: "he",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewLispParser(tt.input)
			expr, err := parser.Parse()
			require.NoError(t, err)

			result, err := ctx.EvalAll(expr)
			require.NoError(t, err)

			if atom, ok := result.(*Atom); ok {
				assert.Equal(t, tt.expected, atom.AsString())
			}
		})
	}
}

func TestBuiltinIf(t *testing.T) {
	ctx := NewEvalContext()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "if true",
			input:    `(if true "yes" "no")`,
			expected: "yes",
		},
		{
			name:     "if false",
			input:    `(if false "yes" "no")`,
			expected: "no",
		},
		{
			name:     "if with comparison",
			input:    `(if (> 5 3) "greater" "lesser")`,
			expected: "greater",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewLispParser(tt.input)
			expr, err := parser.Parse()
			require.NoError(t, err)

			result, err := ctx.EvalAll(expr)
			require.NoError(t, err)

			if atom, ok := result.(*Atom); ok {
				assert.Equal(t, tt.expected, atom.AsString())
			}
		})
	}
}

func TestBuiltinDefault(t *testing.T) {
	ctx := NewEvalContext()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "default with value",
			input:    `(default "hello" "fallback")`,
			expected: "hello",
		},
		{
			name:     "default with empty string",
			input:    `(default "" "fallback")`,
			expected: "fallback",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewLispParser(tt.input)
			expr, err := parser.Parse()
			require.NoError(t, err)

			result, err := ctx.EvalAll(expr)
			require.NoError(t, err)

			if atom, ok := result.(*Atom); ok {
				assert.Equal(t, tt.expected, atom.AsString())
			}
		})
	}
}

func TestBuiltinArithmetic(t *testing.T) {
	ctx := NewEvalContext()

	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{
			name:     "addition",
			input:    `(+ 1 2 3)`,
			expected: 6,
		},
		{
			name:     "subtraction",
			input:    `(- 10 3)`,
			expected: 7,
		},
		{
			name:     "multiplication",
			input:    `(* 2 3 4)`,
			expected: 24,
		},
		{
			name:     "division",
			input:    `(/ 10 2)`,
			expected: 5,
		},
		{
			name:     "modulo",
			input:    `(mod 10 3)`,
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewLispParser(tt.input)
			expr, err := parser.Parse()
			require.NoError(t, err)

			result, err := ctx.EvalAll(expr)
			require.NoError(t, err)

			if atom, ok := result.(*Atom); ok {
				assert.Equal(t, tt.expected, atom.AsInt())
			}
		})
	}
}

func TestBuiltinComparison(t *testing.T) {
	ctx := NewEvalContext()

	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "equal true",
			input:    `(eq "a" "a")`,
			expected: true,
		},
		{
			name:     "equal false",
			input:    `(eq "a" "b")`,
			expected: false,
		},
		{
			name:     "less than",
			input:    `(< 1 2)`,
			expected: true,
		},
		{
			name:     "greater than",
			input:    `(> 5 3)`,
			expected: true,
		},
		{
			name:     "less than or equal",
			input:    `(<= 2 2)`,
			expected: true,
		},
		{
			name:     "greater than or equal",
			input:    `(>= 3 3)`,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewLispParser(tt.input)
			expr, err := parser.Parse()
			require.NoError(t, err)

			result, err := ctx.EvalAll(expr)
			require.NoError(t, err)

			if atom, ok := result.(*Atom); ok {
				assert.Equal(t, tt.expected, atom.AsBool())
			}
		})
	}
}

func TestBuiltinLogical(t *testing.T) {
	ctx := NewEvalContext()

	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "and true",
			input:    `(and true true)`,
			expected: true,
		},
		{
			name:     "and false",
			input:    `(and true false)`,
			expected: false,
		},
		{
			name:     "or true",
			input:    `(or false true)`,
			expected: true,
		},
		{
			name:     "or false",
			input:    `(or false false)`,
			expected: false,
		},
		{
			name:     "not true",
			input:    `(not false)`,
			expected: true,
		},
		{
			name:     "not false",
			input:    `(not true)`,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewLispParser(tt.input)
			expr, err := parser.Parse()
			require.NoError(t, err)

			result, err := ctx.EvalAll(expr)
			require.NoError(t, err)

			if atom, ok := result.(*Atom); ok {
				assert.Equal(t, tt.expected, atom.AsBool())
			}
		})
	}
}

func TestBuiltinBase64(t *testing.T) {
	ctx := NewEvalContext()

	t.Run("encode", func(t *testing.T) {
		parser := NewLispParser(`(base64-encode "hello")`)
		expr, err := parser.Parse()
		require.NoError(t, err)

		result, err := ctx.EvalAll(expr)
		require.NoError(t, err)

		if atom, ok := result.(*Atom); ok {
			assert.Equal(t, "aGVsbG8=", atom.AsString())
		}
	})

	t.Run("decode", func(t *testing.T) {
		parser := NewLispParser(`(base64-decode "aGVsbG8=")`)
		expr, err := parser.Parse()
		require.NoError(t, err)

		result, err := ctx.EvalAll(expr)
		require.NoError(t, err)

		if atom, ok := result.(*Atom); ok {
			assert.Equal(t, "hello", atom.AsString())
		}
	})
}

func TestBuiltinSHA256(t *testing.T) {
	ctx := NewEvalContext()

	parser := NewLispParser(`(sha256 "hello")`)
	expr, err := parser.Parse()
	require.NoError(t, err)

	result, err := ctx.EvalAll(expr)
	require.NoError(t, err)

	if atom, ok := result.(*Atom); ok {
		// SHA256 of "hello" is known
		assert.Equal(t, "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824", atom.AsString())
	}
}

func TestBuiltinUUID(t *testing.T) {
	ctx := NewEvalContext()

	parser := NewLispParser(`(uuid)`)
	expr, err := parser.Parse()
	require.NoError(t, err)

	result, err := ctx.EvalAll(expr)
	require.NoError(t, err)

	if atom, ok := result.(*Atom); ok {
		// UUID should be 36 characters (8-4-4-4-12 format)
		assert.Len(t, atom.AsString(), 36)
		assert.Contains(t, atom.AsString(), "-")
	}
}

func TestBuiltinTime(t *testing.T) {
	ctx := NewEvalContext()

	t.Run("now", func(t *testing.T) {
		parser := NewLispParser(`(now)`)
		expr, err := parser.Parse()
		require.NoError(t, err)

		result, err := ctx.EvalAll(expr)
		require.NoError(t, err)

		if atom, ok := result.(*Atom); ok {
			// Should be RFC3339 format
			assert.Contains(t, atom.AsString(), "T")
			assert.Contains(t, atom.AsString(), "Z")
		}
	})

	t.Run("timestamp", func(t *testing.T) {
		parser := NewLispParser(`(timestamp)`)
		expr, err := parser.Parse()
		require.NoError(t, err)

		result, err := ctx.EvalAll(expr)
		require.NoError(t, err)

		if atom, ok := result.(*Atom); ok {
			// Should be a positive integer
			assert.True(t, atom.AsInt() > 0)
		}
	})
}

func TestBuiltinSystem(t *testing.T) {
	ctx := NewEvalContext()

	t.Run("hostname", func(t *testing.T) {
		parser := NewLispParser(`(hostname)`)
		expr, err := parser.Parse()
		require.NoError(t, err)

		result, err := ctx.EvalAll(expr)
		require.NoError(t, err)

		if atom, ok := result.(*Atom); ok {
			assert.NotEmpty(t, atom.AsString())
		}
	})

	t.Run("home", func(t *testing.T) {
		parser := NewLispParser(`(home)`)
		expr, err := parser.Parse()
		require.NoError(t, err)

		result, err := ctx.EvalAll(expr)
		require.NoError(t, err)

		if atom, ok := result.(*Atom); ok {
			assert.NotEmpty(t, atom.AsString())
			assert.True(t, strings.HasPrefix(atom.AsString(), "/"))
		}
	})

	t.Run("cwd", func(t *testing.T) {
		parser := NewLispParser(`(cwd)`)
		expr, err := parser.Parse()
		require.NoError(t, err)

		result, err := ctx.EvalAll(expr)
		require.NoError(t, err)

		if atom, ok := result.(*Atom); ok {
			assert.NotEmpty(t, atom.AsString())
		}
	})
}

func TestBuiltinList(t *testing.T) {
	ctx := NewEvalContext()

	t.Run("list creation", func(t *testing.T) {
		parser := NewLispParser(`(list 1 2 3)`)
		expr, err := parser.Parse()
		require.NoError(t, err)

		result, err := ctx.EvalAll(expr)
		require.NoError(t, err)

		if list, ok := result.(*List); ok {
			assert.Len(t, list.Items, 3)
		}
	})

	t.Run("first", func(t *testing.T) {
		parser := NewLispParser(`(first (list "a" "b" "c"))`)
		expr, err := parser.Parse()
		require.NoError(t, err)

		result, err := ctx.EvalAll(expr)
		require.NoError(t, err)

		if atom, ok := result.(*Atom); ok {
			assert.Equal(t, "a", atom.AsString())
		}
	})

	t.Run("len", func(t *testing.T) {
		parser := NewLispParser(`(len (list 1 2 3 4 5))`)
		expr, err := parser.Parse()
		require.NoError(t, err)

		result, err := ctx.EvalAll(expr)
		require.NoError(t, err)

		if atom, ok := result.(*Atom); ok {
			assert.Equal(t, 5, atom.AsInt())
		}
	})
}

func TestBuiltinTypeChecks(t *testing.T) {
	ctx := NewEvalContext()

	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "string? true",
			input:    `(string? "hello")`,
			expected: true,
		},
		{
			name:     "number? true",
			input:    `(number? 42)`,
			expected: true,
		},
		{
			name:     "bool? true",
			input:    `(bool? true)`,
			expected: true,
		},
		{
			name:     "list? true",
			input:    `(list? (list 1 2))`,
			expected: true,
		},
		{
			name:     "empty? true for empty string",
			input:    `(empty? "")`,
			expected: true,
		},
		{
			name:     "empty? false for non-empty",
			input:    `(empty? "hello")`,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewLispParser(tt.input)
			expr, err := parser.Parse()
			require.NoError(t, err)

			result, err := ctx.EvalAll(expr)
			require.NoError(t, err)

			if atom, ok := result.(*Atom); ok {
				assert.Equal(t, tt.expected, atom.AsBool())
			}
		})
	}
}

func TestBuiltinLet(t *testing.T) {
	ctx := NewEvalContext()

	t.Run("let binding", func(t *testing.T) {
		parser := NewLispParser(`(let ((x "hello") (y "world")) (concat x " " y))`)
		expr, err := parser.Parse()
		require.NoError(t, err)

		result, err := ctx.EvalAll(expr)
		require.NoError(t, err)

		if atom, ok := result.(*Atom); ok {
			assert.Equal(t, "hello world", atom.AsString())
		}
	})

	t.Run("let scope isolation", func(t *testing.T) {
		ctx.Variables["x"] = "outer"

		parser := NewLispParser(`(let ((x "inner")) x)`)
		expr, err := parser.Parse()
		require.NoError(t, err)

		result, err := ctx.EvalAll(expr)
		require.NoError(t, err)

		if atom, ok := result.(*Atom); ok {
			assert.Equal(t, "inner", atom.AsString())
		}

		// Outer value should be preserved
		assert.Equal(t, "outer", ctx.Variables["x"])
	})
}

func TestBuiltinMatch(t *testing.T) {
	ctx := NewEvalContext()

	t.Run("match? true", func(t *testing.T) {
		parser := NewLispParser(`(match? "^hello" "hello world")`)
		expr, err := parser.Parse()
		require.NoError(t, err)

		result, err := ctx.EvalAll(expr)
		require.NoError(t, err)

		if atom, ok := result.(*Atom); ok {
			assert.True(t, atom.AsBool())
		}
	})

	t.Run("match? false", func(t *testing.T) {
		parser := NewLispParser(`(match? "^world" "hello world")`)
		expr, err := parser.Parse()
		require.NoError(t, err)

		result, err := ctx.EvalAll(expr)
		require.NoError(t, err)

		if atom, ok := result.(*Atom); ok {
			assert.False(t, atom.AsBool())
		}
	})
}

func TestNestedEvaluation(t *testing.T) {
	ctx := NewEvalContext()

	os.Setenv("TEST_PREFIX", "prefix")
	defer os.Unsetenv("TEST_PREFIX")

	t.Run("nested function calls", func(t *testing.T) {
		parser := NewLispParser(`(concat (env "TEST_PREFIX") "-" (upper "suffix"))`)
		expr, err := parser.Parse()
		require.NoError(t, err)

		result, err := ctx.EvalAll(expr)
		require.NoError(t, err)

		if atom, ok := result.(*Atom); ok {
			assert.Equal(t, "prefix-SUFFIX", atom.AsString())
		}
	})

	t.Run("conditional with env", func(t *testing.T) {
		parser := NewLispParser(`(if (env? "TEST_PREFIX") (env "TEST_PREFIX") "default")`)
		expr, err := parser.Parse()
		require.NoError(t, err)

		result, err := ctx.EvalAll(expr)
		require.NoError(t, err)

		if atom, ok := result.(*Atom); ok {
			assert.Equal(t, "prefix", atom.AsString())
		}
	})
}

func TestVariables(t *testing.T) {
	ctx := NewEvalContext()

	t.Run("set and get variable", func(t *testing.T) {
		// Set variable
		parser := NewLispParser(`(set "myvar" "myvalue")`)
		expr, err := parser.Parse()
		require.NoError(t, err)
		_, err = ctx.EvalAll(expr)
		require.NoError(t, err)

		// Get variable
		parser = NewLispParser(`(var "myvar")`)
		expr, err = parser.Parse()
		require.NoError(t, err)
		result, err := ctx.EvalAll(expr)
		require.NoError(t, err)

		if atom, ok := result.(*Atom); ok {
			assert.Equal(t, "myvalue", atom.AsString())
		}
	})
}

func TestBuiltinRange(t *testing.T) {
	ctx := NewEvalContext()

	t.Run("range with end", func(t *testing.T) {
		parser := NewLispParser(`(range 5)`)
		expr, err := parser.Parse()
		require.NoError(t, err)

		result, err := ctx.EvalAll(expr)
		require.NoError(t, err)

		if list, ok := result.(*List); ok {
			assert.Len(t, list.Items, 5)
		}
	})

	t.Run("range with start and end", func(t *testing.T) {
		parser := NewLispParser(`(range 1 4)`)
		expr, err := parser.Parse()
		require.NoError(t, err)

		result, err := ctx.EvalAll(expr)
		require.NoError(t, err)

		if list, ok := result.(*List); ok {
			assert.Len(t, list.Items, 3) // 1, 2, 3
		}
	})
}

func TestBuiltinSplit(t *testing.T) {
	ctx := NewEvalContext()

	parser := NewLispParser(`(split "a,b,c" ",")`)
	expr, err := parser.Parse()
	require.NoError(t, err)

	result, err := ctx.EvalAll(expr)
	require.NoError(t, err)

	if list, ok := result.(*List); ok {
		assert.Len(t, list.Items, 3)
		assert.Equal(t, "a", list.Items[0].(*Atom).AsString())
		assert.Equal(t, "b", list.Items[1].(*Atom).AsString())
		assert.Equal(t, "c", list.Items[2].(*Atom).AsString())
	}
}

func TestBuiltinJoin(t *testing.T) {
	ctx := NewEvalContext()

	parser := NewLispParser(`(join (list "a" "b" "c") "-")`)
	expr, err := parser.Parse()
	require.NoError(t, err)

	result, err := ctx.EvalAll(expr)
	require.NoError(t, err)

	if atom, ok := result.(*Atom); ok {
		assert.Equal(t, "a-b-c", atom.AsString())
	}
}

func TestBuiltinPath(t *testing.T) {
	ctx := NewEvalContext()

	t.Run("dirname", func(t *testing.T) {
		parser := NewLispParser(`(dirname "/path/to/file.txt")`)
		expr, err := parser.Parse()
		require.NoError(t, err)

		result, err := ctx.EvalAll(expr)
		require.NoError(t, err)

		if atom, ok := result.(*Atom); ok {
			assert.Equal(t, "/path/to", atom.AsString())
		}
	})

	t.Run("basename", func(t *testing.T) {
		parser := NewLispParser(`(basename "/path/to/file.txt")`)
		expr, err := parser.Parse()
		require.NoError(t, err)

		result, err := ctx.EvalAll(expr)
		require.NoError(t, err)

		if atom, ok := result.(*Atom); ok {
			assert.Equal(t, "file.txt", atom.AsString())
		}
	})
}
