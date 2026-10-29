package builtin

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/expr-lang/expr/internal/deref"
	"github.com/expr-lang/expr/vm/runtime"
)

var (
	// MaxDepth limits the recursion depth for nested structures.
	MaxDepth      = 10000
	ErrorMaxDepth = errors.New("recursion depth exceeded")
)

var Builtins = []*Function{
	{
		Name:      "all",
		Predicate: true,
		Types:     types(new(func([]any, func(any) bool) bool)),
		Arguments: []ArgumentSpec{
			{Kind: ArgumentExpression},
			{Kind: ArgumentPredicate},
		},
		Category:    CategoryArray,
		Description: "Returns **true** if all elements satisfies the [predicate](#predicate).\nIf the array is empty, returns **true**.",
		Signature:   "all(array, predicate)",
		Examples: []string{
			`all([1, 2, 3], # > 0) == true`,
		},
	},
	{
		Name:      "none",
		Predicate: true,
		Types:     types(new(func([]any, func(any) bool) bool)),
		Arguments: []ArgumentSpec{
			{Kind: ArgumentExpression},
			{Kind: ArgumentPredicate},
		},
		Category:    CategoryArray,
		Description: "Returns **true** if _all elements does not_ satisfy the [predicate](#predicate).\nIf the array is empty, returns **true**.",
		Signature:   "none(array, predicate)",
		Examples: []string{
			`none([1, 2, 3], # > 3) == true`,
		},
	},
	{
		Name:      "any",
		Predicate: true,
		Types:     types(new(func([]any, func(any) bool) bool)),
		Arguments: []ArgumentSpec{
			{Kind: ArgumentExpression},
			{Kind: ArgumentPredicate},
		},
		Category:    CategoryArray,
		Description: "Returns **true** if any elements satisfies the [predicate](#predicate).\nIf the array is empty, returns **false**.",
		Signature:   "any(array, predicate)",
		Examples: []string{
			`any([1, 2, 3], # > 2) == true`,
		},
	},
	{
		Name:      "one",
		Predicate: true,
		Types:     types(new(func([]any, func(any) bool) bool)),
		Arguments: []ArgumentSpec{
			{Kind: ArgumentExpression},
			{Kind: ArgumentPredicate},
		},
		Category:    CategoryArray,
		Description: "Returns **true** if _exactly one_ element satisfies the [predicate](#predicate).\nIf the array is empty, returns **false**.",
		Signature:   "one(array, predicate)",
		Examples: []string{
			`one([1, 2, 3], # == 2) == true`,
		},
	},
	{
		Name:      "filter",
		Predicate: true,
		Types:     types(new(func([]any, func(any) bool) []any)),
		Arguments: []ArgumentSpec{
			{Kind: ArgumentExpression},
			{Kind: ArgumentPredicate},
		},
		Category:    CategoryArray,
		Description: "Returns new array by filtering elements of array by [predicate](#predicate).",
		Signature:   "filter(array, predicate)",
		Examples: []string{
			`filter([1, 2, 3, 4], # > 2) == [3, 4]`,
		},
	},
	{
		Name:      "map",
		Predicate: true,
		Types:     types(new(func([]any, func(any) any) []any)),
		Arguments: []ArgumentSpec{
			{Kind: ArgumentExpression},
			{Kind: ArgumentPredicate},
		},
		Category:    CategoryArray,
		Description: "Returns new array by applying the [predicate](#predicate) to each element of\nthe array.",
		Signature:   "map(array, predicate)",
		Examples: []string{
			`map([1, 2, 3], # * 2) == [2, 4, 6]`,
		},
	},
	{
		Name:      "find",
		Predicate: true,
		Types:     types(new(func([]any, func(any) bool) any)),
		Arguments: []ArgumentSpec{
			{Kind: ArgumentExpression},
			{Kind: ArgumentPredicate},
		},
		Category:    CategoryArray,
		Description: "Finds the first element in an array that satisfies the [predicate](#predicate).",
		Signature:   "find(array, predicate)",
		Examples: []string{
			`find([1, 2, 3, 4], # > 2) == 3`,
		},
	},
	{
		Name:      "findIndex",
		Predicate: true,
		Types:     types(new(func([]any, func(any) bool) int)),
		Arguments: []ArgumentSpec{
			{Kind: ArgumentExpression},
			{Kind: ArgumentPredicate},
		},
		Category:    CategoryArray,
		Description: "Finds the index of the first element in an array that satisfies the [predicate](#predicate).",
		Signature:   "findIndex(array, predicate)",
		Examples: []string{
			`findIndex([1, 2, 3, 4], # > 2) == 2`,
		},
	},
	{
		Name:      "findLast",
		Predicate: true,
		Types:     types(new(func([]any, func(any) bool) any)),
		Arguments: []ArgumentSpec{
			{Kind: ArgumentExpression},
			{Kind: ArgumentPredicate},
		},
		Category:    CategoryArray,
		Description: "Finds the last element in an array that satisfies the [predicate](#predicate).",
		Signature:   "findLast(array, predicate)",
		Examples: []string{
			`findLast([1, 2, 3, 4], # > 2) == 4`,
		},
	},
	{
		Name:      "findLastIndex",
		Predicate: true,
		Types:     types(new(func([]any, func(any) bool) int)),
		Arguments: []ArgumentSpec{
			{Kind: ArgumentExpression},
			{Kind: ArgumentPredicate},
		},
		Category:    CategoryArray,
		Description: "Finds the index of the last element in an array that satisfies the [predicate](#predicate).",
		Signature:   "findLastIndex(array, predicate)",
		Examples: []string{
			`findLastIndex([1, 2, 3, 4], # > 2) == 3`,
		},
	},
	{
		Name:      "count",
		Predicate: true,
		Types:     types(new(func([]any, func(any) bool) int)),
		Arguments: []ArgumentSpec{
			{Kind: ArgumentExpression},
			{Kind: ArgumentPredicate, Optional: true},
		},
		Category: CategoryArray,
		Description: "Returns the number of elements what satisfies the [predicate](#predicate).\n" +
			"If the predicate is not given, returns the number of `true` elements in the array.",
		Signature: "count(array[, predicate])",
		Examples: []string{
			`count([1, 2, 3], # > 1) == 2`,
			`len(filter([1, 2, 3], # > 1)) == 2`,
			`count([true, false, true]) == 2`,
		},
	},
	{
		Name:      "sum",
		Predicate: true,
		Types:     types(new(func([]any, func(any) bool) int)),
		Arguments: []ArgumentSpec{
			{Kind: ArgumentExpression},
			{Kind: ArgumentPredicate, Optional: true},
		},
		Category: CategoryArray,
		Description: "Returns the sum of all numbers in the array.\n" +
			"If the optional `predicate` argument is given, it is a predicate that is applied on each element\n" +
			"of the array before summing.",
		Signature: "sum(array[, predicate])",
		Examples: []string{
			`sum([1, 2, 3]) == 6`,
			`sum(map([1, 2, 3], # * 2)) == 12`,
		},
	},
	{
		Name:      "groupBy",
		Predicate: true,
		Types:     types(new(func([]any, func(any) any) map[any][]any)),
		Arguments: []ArgumentSpec{
			{Kind: ArgumentExpression},
			{Kind: ArgumentPredicate},
		},
		Category:    CategoryArray,
		Description: "Groups the elements of an array by the result of the [predicate](#predicate).",
		Signature:   "groupBy(array, predicate)",
		Examples: []string{
			`groupBy([1, 2, 3, 4], # % 2)[0] == [2, 4]`,
		},
	},
	{
		Name:      "sortBy",
		Predicate: true,
		Types:     types(new(func([]any, func(any) bool, string) []any)),
		Arguments: []ArgumentSpec{
			{Kind: ArgumentExpression},
			{Kind: ArgumentPredicate},
			{Kind: ArgumentExpression, Optional: true},
		},
		Category: CategoryArray,
		Description: "Sorts an array by the result of the [predicate](#predicate). Optional `order` argument can be used to specify the order\n" +
			"of sorting: `asc` or `desc`.",
		Signature: "sortBy(array[, predicate, order])",
		Examples: []string{
			`sortBy([3, 1, 2], #) == [1, 2, 3]`,
			`sortBy([3, 1, 2], #, "desc") == [3, 2, 1]`,
		},
	},
	{
		Name:      "reduce",
		Predicate: true,
		Types:     types(new(func([]any, func(any, any) any, any) any)),
		Arguments: []ArgumentSpec{
			{Kind: ArgumentExpression},
			{Kind: ArgumentPredicate},
			{Kind: ArgumentExpression, Optional: true},
		},
		Category: CategoryArray,
		Description: "Applies a predicate to each element in the array, reducing the array to a single value.\n" +
			"Optional `initialValue` argument can be used to specify the initial value of the accumulator.\n" +
			"If `initialValue` is not given, the first element of the array is used as the initial value.\n\n" +
			"Following variables are available in the predicate:\n\n" +
			"- `#` - the current element\n" +
			"- `#acc` - the accumulator\n" +
			"- `#index` - the index of the current element",
		Signature: "reduce(array, predicate[, initialValue])",
		Examples: []string{
			`reduce(1..9, #acc + #)`,
			`reduce(1..9, #acc + #, 0)`,
		},
	},
	{
		Name: "len",
		Fast: Len,
		Validate: func(args []reflect.Type) (reflect.Type, error) {
			if len(args) != 1 {
				return anyType, fmt.Errorf("invalid number of arguments (expected 1, got %d)", len(args))
			}
			switch kind(args[0]) {
			case reflect.Array, reflect.Map, reflect.Slice, reflect.String, reflect.Interface:
				return integerType, nil
			}
			return anyType, fmt.Errorf("invalid argument for len (type %s)", args[0])
		},
		Category:    CategoryMiscellaneous,
		Description: "Returns the length of an array, a map or a string.",
		Signature:   "len(v)",
		Examples: []string{
			`len([1, 2, 3]) == 3`,
			`len({"name": "John", "age": 30}) == 2`,
			`len("Hello") == 5`,
		},
	},
	{
		Name:     "type",
		Fast:     Type,
		Types:    types(new(func(any) string)),
		Category: CategoryConversion,
		Description: "Returns the type of the given value `v`.\n\n" +
			"Returns on of the following types:\n\n" +
			"- `nil`\n" +
			"- `bool`\n" +
			"- `int`\n" +
			"- `uint`\n" +
			"- `float`\n" +
			"- `string`\n" +
			"- `array`\n" +
			"- `map`.\n\n" +
			"For named types and structs, the type name is returned.",
		Signature: "type(v)",
		Examples: []string{
			`type(42) == "int"`,
			`type("hello") == "string"`,
			`type(now()) == "time.Time"`,
		},
	},
	{
		Name: "abs",
		Fast: Abs,
		Validate: func(args []reflect.Type) (reflect.Type, error) {
			if len(args) != 1 {
				return anyType, fmt.Errorf("invalid number of arguments (expected 1, got %d)", len(args))
			}
			switch kind(args[0]) {
			case reflect.Float32, reflect.Float64, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Interface:
				return args[0], nil
			}
			return anyType, fmt.Errorf("invalid argument for abs (type %s)", args[0])
		},
		Category:    CategoryNumber,
		Description: "Returns the absolute value of a number.",
		Signature:   "abs(n)",
		Examples: []string{
			`abs(-5) == 5`,
		},
	},
	{
		Name: "ceil",
		Fast: Ceil,
		Validate: func(args []reflect.Type) (reflect.Type, error) {
			return validateRoundFunc("ceil", args)
		},
		Category:    CategoryNumber,
		Description: "Returns the least integer value greater than or equal to x.",
		Signature:   "ceil(n)",
		Examples: []string{
			`ceil(1.5) == 2.0`,
		},
	},
	{
		Name: "floor",
		Fast: Floor,
		Validate: func(args []reflect.Type) (reflect.Type, error) {
			return validateRoundFunc("floor", args)
		},
		Category:    CategoryNumber,
		Description: "Returns the greatest integer value less than or equal to x.",
		Signature:   "floor(n)",
		Examples: []string{
			`floor(1.5) == 1.0`,
		},
	},
	{
		Name: "round",
		Fast: Round,
		Validate: func(args []reflect.Type) (reflect.Type, error) {
			return validateRoundFunc("round", args)
		},
		Category:    CategoryNumber,
		Description: "Returns the nearest integer, rounding half away from zero.",
		Signature:   "round(n)",
		Examples: []string{
			`round(1.5) == 2.0`,
		},
	},
	{
		Name: "int",
		Fast: Int,
		Validate: func(args []reflect.Type) (reflect.Type, error) {
			if len(args) != 1 {
				return anyType, fmt.Errorf("invalid number of arguments (expected 1, got %d)", len(args))
			}
			switch kind(args[0]) {
			case reflect.Interface:
				return integerType, nil
			case reflect.Float32, reflect.Float64, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				return integerType, nil
			case reflect.String:
				return integerType, nil
			}
			return anyType, fmt.Errorf("invalid argument for int (type %s)", args[0])
		},
		Category:    CategoryConversion,
		Description: "Returns the integer value of a number or a string.",
		Signature:   "int(v)",
		Examples: []string{
			`int("123") == 123`,
		},
	},
	{
		Name: "float",
		Fast: Float,
		Validate: func(args []reflect.Type) (reflect.Type, error) {
			if len(args) != 1 {
				return anyType, fmt.Errorf("invalid number of arguments (expected 1, got %d)", len(args))
			}
			switch kind(args[0]) {
			case reflect.Interface:
				return floatType, nil
			case reflect.Float32, reflect.Float64, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				return floatType, nil
			case reflect.String:
				return floatType, nil
			}
			return anyType, fmt.Errorf("invalid argument for float (type %s)", args[0])
		},
		Category:    CategoryConversion,
		Description: "Returns the float value of a number or a string.",
		Signature:   "float(v)",
		Examples: []string{
			`float("123.45") == 123.45`,
		},
	},
	{
		Name:        "string",
		Fast:        String,
		Types:       types(new(func(any any) string)),
		Category:    CategoryConversion,
		Description: "Converts the given value `v` into a string representation.",
		Signature:   "string(v)",
		Examples: []string{
			`string(123) == "123"`,
		},
	},
	{
		Name: "trim",
		Func: func(args ...any) (any, error) {
			if len(args) == 1 {
				return strings.TrimSpace(args[0].(string)), nil
			} else if len(args) == 2 {
				return strings.Trim(args[0].(string), args[1].(string)), nil
			} else {
				return nil, fmt.Errorf("invalid number of arguments for trim (expected 1 or 2, got %d)", len(args))
			}
		},
		Types: types(
			strings.TrimSpace,
			strings.Trim,
		),
		Category: CategoryString,
		Description: "Removes white spaces from both ends of a string `str`.\n" +
			"If the optional `chars` argument is given, it is a string specifying the set of characters to be removed.",
		Signature: "trim(str[, chars])",
		Examples: []string{
			`trim("  Hello  ") == "Hello"`,
			`trim("__Hello__", "_") == "Hello"`,
		},
	},
	{
		Name: "trimPrefix",
		Func: func(args ...any) (any, error) {
			s := " "
			if len(args) == 2 {
				s = args[1].(string)
			}
			return strings.TrimPrefix(args[0].(string), s), nil
		},
		Types: types(
			strings.TrimPrefix,
			new(func(string) string),
		),
		Category:    CategoryString,
		Description: "Removes the specified prefix from the string `str` if it starts with that prefix.",
		Signature:   "trimPrefix(str, prefix)",
		Examples: []string{
			`trimPrefix("HelloWorld", "Hello") == "World"`,
		},
	},
	{
		Name: "trimSuffix",
		Func: func(args ...any) (any, error) {
			s := " "
			if len(args) == 2 {
				s = args[1].(string)
			}
			return strings.TrimSuffix(args[0].(string), s), nil
		},
		Types: types(
			strings.TrimSuffix,
			new(func(string) string),
		),
		Category:    CategoryString,
		Description: "Removes the specified suffix from the string `str` if it ends with that suffix.",
		Signature:   "trimSuffix(str, suffix)",
		Examples: []string{
			`trimSuffix("HelloWorld", "World") == "Hello"`,
		},
	},
	{
		Name: "upper",
		Fast: func(arg any) any {
			return strings.ToUpper(arg.(string))
		},
		Types:       types(strings.ToUpper),
		Category:    CategoryString,
		Description: "Converts all the characters in string `str` to uppercase.",
		Signature:   "upper(str)",
		Examples: []string{
			`upper("hello") == "HELLO"`,
		},
	},
	{
		Name: "lower",
		Fast: func(arg any) any {
			return strings.ToLower(arg.(string))
		},
		Types:       types(strings.ToLower),
		Category:    CategoryString,
		Description: "Converts all the characters in string `str` to lowercase.",
		Signature:   "lower(str)",
		Examples: []string{
			`lower("HELLO") == "hello"`,
		},
	},
	{
		Name: "split",
		Func: func(args ...any) (any, error) {
			if len(args) == 2 {
				return strings.Split(args[0].(string), args[1].(string)), nil
			} else if len(args) == 3 {
				return strings.SplitN(args[0].(string), args[1].(string), runtime.ToInt(args[2])), nil
			} else {
				return nil, fmt.Errorf("invalid number of arguments for split (expected 2 or 3, got %d)", len(args))
			}
		},
		Types: types(
			strings.Split,
			strings.SplitN,
		),
		Category:    CategoryString,
		Description: "Splits the string `str` at each instance of the delimiter and returns an array of substrings.",
		Signature:   "split(str, delimiter[, n])",
		Examples: []string{
			`split("apple,orange,grape", ",") == ["apple", "orange", "grape"]`,
			`split("apple,orange,grape", ",", 2) == ["apple", "orange,grape"]`,
		},
	},
	{
		Name: "splitAfter",
		Func: func(args ...any) (any, error) {
			if len(args) == 2 {
				return strings.SplitAfter(args[0].(string), args[1].(string)), nil
			} else if len(args) == 3 {
				return strings.SplitAfterN(args[0].(string), args[1].(string), runtime.ToInt(args[2])), nil
			} else {
				return nil, fmt.Errorf("invalid number of arguments for splitAfter (expected 2 or 3, got %d)", len(args))
			}
		},
		Types: types(
			strings.SplitAfter,
			strings.SplitAfterN,
		),
		Category:    CategoryString,
		Description: "Splits the string `str` after each instance of the delimiter.",
		Signature:   "splitAfter(str, delimiter[, n])",
		Examples: []string{
			`splitAfter("apple,orange,grape", ",") == ["apple,", "orange,", "grape"]`,
			`splitAfter("apple,orange,grape", ",", 2) == ["apple,", "orange,grape"]`,
		},
	},
	{
		Name: "replace",
		Func: func(args ...any) (any, error) {
			if len(args) == 4 {
				return strings.Replace(args[0].(string), args[1].(string), args[2].(string), runtime.ToInt(args[3])), nil
			} else if len(args) == 3 {
				return strings.ReplaceAll(args[0].(string), args[1].(string), args[2].(string)), nil
			} else {
				return nil, fmt.Errorf("invalid number of arguments for replace (expected 3 or 4, got %d)", len(args))
			}
		},
		Types: types(
			strings.Replace,
			strings.ReplaceAll,
		),
		Category:    CategoryString,
		Description: "Replaces all occurrences of `old` in string `str` with `new`.",
		Signature:   "replace(str, old, new)",
		Examples: []string{
			`replace("Hello World", "World", "Universe") == "Hello Universe"`,
		},
	},
	{
		Name: "repeat",
		Safe: func(args ...any) (any, uint, error) {
			s := args[0].(string)
			n := runtime.ToInt(args[1])
			if n < 0 {
				return nil, 0, fmt.Errorf("invalid argument for repeat (expected positive integer, got %d)", n)
			}
			if n > 1e6 {
				return nil, 0, fmt.Errorf("memory budget exceeded")
			}
			return strings.Repeat(s, n), uint(len(s) * n), nil
		},
		Types:       types(strings.Repeat),
		Category:    CategoryString,
		Description: "Repeats the string `str` `n` times.",
		Signature:   "repeat(str, n)",
		Examples: []string{
			`repeat("Hi", 3) == "HiHiHi"`,
		},
	},
	{
		Name: "join",
		Func: func(args ...any) (any, error) {
			glue := ""
			if len(args) == 2 {
				glue = args[1].(string)
			}
			switch args[0].(type) {
			case []string:
				return strings.Join(args[0].([]string), glue), nil
			case []any:
				var s []string
				for _, arg := range args[0].([]any) {
					s = append(s, arg.(string))
				}
				return strings.Join(s, glue), nil
			}
			return nil, fmt.Errorf("invalid argument for join (type %s)", reflect.TypeOf(args[0]))
		},
		Types: types(
			strings.Join,
			new(func([]any, string) string),
			new(func([]any) string),
			new(func([]string, string) string),
			new(func([]string) string),
		),
		Category: CategoryArray,
		Description: "Joins an array of strings into a single string with the given delimiter.\n" +
			"If no delimiter is given, an empty string is used.",
		Signature: "join(array[, delimiter])",
		Examples: []string{
			`join(["apple", "orange", "grape"], ",") == "apple,orange,grape"`,
			`join(["apple", "orange", "grape"]) == "appleorangegrape"`,
		},
	},
	{
		Name: "indexOf",
		Func: func(args ...any) (any, error) {
			return strings.Index(args[0].(string), args[1].(string)), nil
		},
		Types:       types(strings.Index),
		Category:    CategoryString,
		Description: "Returns the index of the first occurrence of the substring in string `str` or -1 if not found.",
		Signature:   "indexOf(str, substring)",
		Examples: []string{
			`indexOf("apple pie", "pie") == 6`,
		},
	},
	{
		Name: "lastIndexOf",
		Func: func(args ...any) (any, error) {
			return strings.LastIndex(args[0].(string), args[1].(string)), nil
		},
		Types:       types(strings.LastIndex),
		Category:    CategoryString,
		Description: "Returns the index of the last occurrence of the substring in string `str` or -1 if not found.",
		Signature:   "lastIndexOf(str, substring)",
		Examples: []string{
			`lastIndexOf("apple pie apple", "apple") == 10`,
		},
	},
	{
		Name: "hasPrefix",
		Func: func(args ...any) (any, error) {
			return strings.HasPrefix(args[0].(string), args[1].(string)), nil
		},
		Types:       types(strings.HasPrefix),
		Category:    CategoryString,
		Description: "Returns `true` if string `str` starts with the given prefix.",
		Signature:   "hasPrefix(str, prefix)",
		Examples: []string{
			`hasPrefix("HelloWorld", "Hello") == true`,
		},
	},
	{
		Name: "hasSuffix",
		Func: func(args ...any) (any, error) {
			return strings.HasSuffix(args[0].(string), args[1].(string)), nil
		},
		Types:       types(strings.HasSuffix),
		Category:    CategoryString,
		Description: "Returns `true` if string `str` ends with the given suffix.",
		Signature:   "hasSuffix(str, suffix)",
		Examples: []string{
			`hasSuffix("HelloWorld", "World") == true`,
		},
	},
	{
		Name: "max",
		Func: func(args ...any) (any, error) {
			return minMax("max", runtime.Less, 0, args...)
		},
		Validate: func(args []reflect.Type) (reflect.Type, error) {
			return validateAggregateFunc("max", args)
		},
		Category:    CategoryNumber,
		Description: "Returns the maximum of the two numbers `n1` and `n2`.",
		Signature:   "max(n1, n2)",
		Examples: []string{
			`max(5, 7) == 7`,
		},
	},
	{
		Name: "min",
		Func: func(args ...any) (any, error) {
			return minMax("min", runtime.More, 0, args...)
		},
		Validate: func(args []reflect.Type) (reflect.Type, error) {
			return validateAggregateFunc("min", args)
		},
		Category:    CategoryNumber,
		Description: "Returns the minimum of the two numbers `n1` and `n2`.",
		Signature:   "min(n1, n2)",
		Examples: []string{
			`min(5, 7) == 5`,
		},
	},
	{
		Name: "mean",
		Func: func(args ...any) (any, error) {
			count, sum, err := mean(0, args...)
			if err != nil {
				return nil, err
			}
			if count == 0 {
				return 0.0, nil
			}
			return sum / float64(count), nil
		},
		Validate: func(args []reflect.Type) (reflect.Type, error) {
			return validateAggregateFunc("mean", args)
		},
		Category:    CategoryArray,
		Description: "Returns the average of all numbers in the array.",
		Signature:   "mean(array)",
		Examples: []string{
			`mean([1, 2, 3]) == 2.0`,
		},
	},
	{
		Name: "median",
		Func: func(args ...any) (any, error) {
			values, err := median(0, args...)
			if err != nil {
				return nil, err
			}
			if n := len(values); n > 0 {
				sort.Float64s(values)
				if n%2 == 1 {
					return values[n/2], nil
				}
				return (values[n/2-1] + values[n/2]) / 2, nil
			}
			return 0.0, nil
		},
		Validate: func(args []reflect.Type) (reflect.Type, error) {
			return validateAggregateFunc("median", args)
		},
		Category:    CategoryArray,
		Description: "Returns the median of all numbers in the array.",
		Signature:   "median(array)",
		Examples: []string{
			`median([1, 2, 3]) == 2.0`,
		},
	},
	{
		Name: "toJSON",
		Func: func(args ...any) (any, error) {
			b, err := json.MarshalIndent(args[0], "", "  ")
			if err != nil {
				return nil, err
			}
			return string(b), nil
		},
		Types: types(new(func(any) string)),
		Category:    CategoryConversion,
		Description: "Converts the given value `v` to its JSON string representation.",
		Signature:   "toJSON(v)",
		Examples: []string{
			`toJSON({"name": "John", "age": 30})`,
		},
	},
	{
		Name: "fromJSON",
		Func: func(args ...any) (any, error) {
			var v any
			err := json.Unmarshal([]byte(args[0].(string)), &v)
			if err != nil {
				return nil, err
			}
			return v, nil
		},
		Types: types(new(func(string) any)),
		Category:    CategoryConversion,
		Description: "Parses the given JSON string `v` and returns the corresponding value.",
		Signature:   "fromJSON(v)",
		Examples: []string{
			`fromJSON('{"name": "John", "age": 30}')`,
		},
	},
	{
		Name: "toBase64",
		Func: func(args ...any) (any, error) {
			return base64.StdEncoding.EncodeToString([]byte(args[0].(string))), nil
		},
		Types: types(new(func(string) string)),
		Category:    CategoryConversion,
		Description: "Encodes the string `v` into Base64 format.",
		Signature:   "toBase64(v)",
		Examples: []string{
			`toBase64("Hello World") == "SGVsbG8gV29ybGQ="`,
		},
	},
	{
		Name: "fromBase64",
		Func: func(args ...any) (any, error) {
			b, err := base64.StdEncoding.DecodeString(args[0].(string))
			if err != nil {
				return nil, err
			}
			return string(b), nil
		},
		Types: types(new(func(string) string)),
		Category:    CategoryConversion,
		Description: "Decodes the Base64 encoded string `v` back to its original form.",
		Signature:   "fromBase64(v)",
		Examples: []string{
			`fromBase64("SGVsbG8gV29ybGQ=") == "Hello World"`,
		},
	},
	{
		Name: "now",
		Func: func(args ...any) (any, error) {
			if len(args) == 0 {
				return time.Now(), nil
			}
			if len(args) == 1 {
				if tz, ok := args[0].(*time.Location); ok {
					return time.Now().In(tz), nil
				}
			}
			return nil, fmt.Errorf("invalid number of arguments (expected 0, got %d)", len(args))
		},
		Validate: func(args []reflect.Type) (reflect.Type, error) {
			if len(args) == 0 {
				return timeType, nil
			}
			if len(args) == 1 {
				if args[0] != nil && args[0].AssignableTo(locationType) {
					return timeType, nil
				}
			}
			return anyType, fmt.Errorf("invalid number of arguments (expected 0, got %d)", len(args))
		},
		Deref: func(i int, arg reflect.Type) bool {
			return false
		},
		Category:    CategoryDate,
		Description: "Returns the current date as a [time.Time](https://pkg.go.dev/time#Time) value.",
		Signature:   "now()",
		Examples: []string{
			`now().Year() >= 2024`,
		},
	},
	{
		Name: "duration",
		Func: func(args ...any) (any, error) {
			return time.ParseDuration(args[0].(string))
		},
		Types: types(time.ParseDuration),
		Category: CategoryDate,
		Description: "Returns [time.Duration](https://pkg.go.dev/time#Duration) value of the given string `str`.\n\n" +
			"Valid time units are \"ns\", \"us\" (or \"µs\"), \"ms\", \"s\", \"m\", \"h\".",
		Signature: "duration(str)",
		Examples: []string{
			`duration("1h").Seconds() == 3600`,
		},
	},
	{
		Name: "date",
		Func: func(args ...any) (any, error) {
			tz, ok := args[0].(*time.Location)
			if ok {
				args = args[1:]
			}

			date := args[0].(string)
			if len(args) == 2 {
				layout := args[1].(string)
				if tz != nil {
					return time.ParseInLocation(layout, date, tz)
				}
				return time.Parse(layout, date)
			}
			if len(args) == 3 {
				layout := args[1].(string)
				timeZone := args[2].(string)
				tz, err := time.LoadLocation(timeZone)
				if err != nil {
					return nil, fmt.Errorf("unknown time zone %s", timeZone)
				}
				t, err := time.ParseInLocation(layout, date, tz)
				if err != nil {
					return nil, err
				}
				return t, nil
			}

			layouts := []string{
				"2006-01-02",
				"15:04:05",
				"2006-01-02 15:04:05",
				time.RFC3339,
				time.RFC822,
				time.RFC850,
				time.RFC1123,
			}
			for _, layout := range layouts {
				if tz == nil {
					t, err := time.Parse(layout, date)
					if err == nil {
						return t, nil
					}
				} else {
					t, err := time.ParseInLocation(layout, date, tz)
					if err == nil {
						return t, nil
					}
				}
			}
			return nil, fmt.Errorf("invalid date %s", date)
		},
		Validate: func(args []reflect.Type) (reflect.Type, error) {
			if len(args) < 1 {
				return anyType, fmt.Errorf("invalid number of arguments (expected at least 1, got %d)", len(args))
			}
			if args[0] != nil && args[0].AssignableTo(locationType) {
				args = args[1:]
			}
			if len(args) > 3 {
				return anyType, fmt.Errorf("invalid number of arguments (expected at most 3, got %d)", len(args))
			}
			return timeType, nil
		},
		Deref: func(i int, arg reflect.Type) bool {
			if arg.AssignableTo(locationType) {
				return false
			}
			return true
		},
		Category: CategoryDate,
		Description: "Converts the given string `str` into a date representation.\n\n" +
			"If the optional `format` argument is given, it is a string specifying the format of the date.\n" +
			"The format string uses the same formatting rules as the standard\n" +
			"Go [time package](https://pkg.go.dev/time#pkg-constants).\n\n" +
			"If the optional `timezone` argument is given, it is a string specifying the timezone of the date.\n\n" +
			"If the `format` argument is not given, the `v` argument must be in one of the following formats:\n\n" +
			"- 2006-01-02\n" +
			"- 15:04:05\n" +
			"- 2006-01-02 15:04:05\n" +
			"- RFC3339\n" +
			"- RFC822,\n" +
			"- RFC850,\n" +
			"- RFC1123,\n\n" +
			"Available methods on the date:\n\n" +
			"- `Year()` - returns the year \n" +
			"- `Month()` - returns the month (starting from 1)\n" +
			"- `Day()` - returns the day of the month\n" +
			"- `Hour()` - returns the hour\n" +
			"- `Minute()` - returns the minute\n" +
			"- `Second()` - returns the second\n" +
			"- `Weekday()` - returns the day of the week\n" +
			"- `YearDay()` - returns the day of the year\n" +
			"- and [more](https://pkg.go.dev/time#Time).",
		Signature: "date(str[, format[, timezone]])",
		Examples: []string{
			`date("2023-08-14")`,
			`date("15:04:05")`,
			`date("2023-08-14T00:00:00Z")`,
			`date("2023-08-14 00:00:00", "2006-01-02 15:04:05", "Europe/Zurich")`,
			`date("2023-08-14").Year() == 2023`,
		},
	},
	{
		Name: "timezone",
		Func: func(args ...any) (any, error) {
			tz, err := time.LoadLocation(args[0].(string))
			if err != nil {
				return nil, err
			}
			return tz, nil
		},
		Types: types(time.LoadLocation),
		Category: CategoryDate,
		Description: "Returns the timezone of the given string `str`. List of available timezones can be\n" +
			"found [here](https://en.wikipedia.org/wiki/List_of_tz_database_time_zones).",
		Signature: "timezone(str)",
		Examples: []string{
			`timezone("Europe/Zurich")`,
			`timezone("UTC")`,
		},
	},
	{
		Name: "first",
		Func: func(args ...any) (any, error) {
			defer func() {
				if r := recover(); r != nil {
					return
				}
			}()
			return runtime.Fetch(args[0], 0), nil
		},
		Validate: func(args []reflect.Type) (reflect.Type, error) {
			if len(args) != 1 {
				return anyType, fmt.Errorf("invalid number of arguments (expected 1, got %d)", len(args))
			}
			switch kind(args[0]) {
			case reflect.Interface:
				return anyType, nil
			case reflect.Slice, reflect.Array:
				return args[0].Elem(), nil
			}
			return anyType, fmt.Errorf("cannot get first element from %s", args[0])
		},
		Category:    CategoryArray,
		Description: "Returns the first element from an array. If the array is empty, returns `nil`.",
		Signature:   "first(array)",
		Examples: []string{
			`first([1, 2, 3]) == 1`,
		},
	},
	{
		Name: "last",
		Func: func(args ...any) (any, error) {
			defer func() {
				if r := recover(); r != nil {
					return
				}
			}()
			return runtime.Fetch(args[0], -1), nil
		},
		Validate: func(args []reflect.Type) (reflect.Type, error) {
			if len(args) != 1 {
				return anyType, fmt.Errorf("invalid number of arguments (expected 1, got %d)", len(args))
			}
			switch kind(args[0]) {
			case reflect.Interface:
				return anyType, nil
			case reflect.Slice, reflect.Array:
				return args[0].Elem(), nil
			}
			return anyType, fmt.Errorf("cannot get last element from %s", args[0])
		},
		Category:    CategoryArray,
		Description: "Returns the last element from an array. If the array is empty, returns `nil`.",
		Signature:   "last(array)",
		Examples: []string{
			`last([1, 2, 3]) == 3`,
		},
	},
	{
		Name: "get",
		Func: get,
		Category: CategoryMiscellaneous,
		Description: "Retrieves the element at the specified index from an array or map `v`. If the index is out of range, returns `nil`.\n" +
			"Or the key does not exist, returns `nil`.",
		Signature: "get(v, index)",
		Examples: []string{
			`get([1, 2, 3], 1) == 2`,
			`get({"name": "John", "age": 30}, "name") == "John"`,
		},
	},
	{
		Name: "take",
		Func: func(args ...any) (any, error) {
			if len(args) != 2 {
				return nil, fmt.Errorf("invalid number of arguments (expected 2, got %d)", len(args))
			}
			v := reflect.ValueOf(args[0])
			if v.Kind() != reflect.Slice && v.Kind() != reflect.Array {
				return nil, fmt.Errorf("cannot take from %s", v.Kind())
			}
			n := reflect.ValueOf(args[1])
			if !n.CanInt() {
				return nil, fmt.Errorf("cannot take %s elements", n.Kind())
			}
			to := 0
			if n.Int() > int64(v.Len()) {
				to = v.Len()
			} else {
				to = int(n.Int())
			}
			return v.Slice(0, to).Interface(), nil
		},
		Validate: func(args []reflect.Type) (reflect.Type, error) {
			if len(args) != 2 {
				return anyType, fmt.Errorf("invalid number of arguments (expected 2, got %d)", len(args))
			}
			switch kind(args[0]) {
			case reflect.Interface, reflect.Slice, reflect.Array:
			default:
				return anyType, fmt.Errorf("cannot take from %s", args[0])
			}
			switch kind(args[1]) {
			case reflect.Interface, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			default:
				return anyType, fmt.Errorf("cannot take %s elements", args[1])
			}
			return args[0], nil
		},
		Category: CategoryArray,
		Description: "Returns the first `n` elements from an array. If the array has fewer than `n` elements, returns the whole array.",
		Signature:   "take(array, n)",
		Examples: []string{
			`take([1, 2, 3, 4], 2) == [1, 2]`,
		},
	},
	{
		Name: "keys",
		Func: func(args ...any) (any, error) {
			if len(args) != 1 {
				return nil, fmt.Errorf("invalid number of arguments (expected 1, got %d)", len(args))
			}
			v := reflect.ValueOf(args[0])
			if v.Kind() != reflect.Map {
				return nil, fmt.Errorf("cannot get keys from %s", v.Kind())
			}
			keys := v.MapKeys()
			out := make([]any, len(keys))
			for i, key := range keys {
				out[i] = key.Interface()
			}
			return out, nil
		},
		Validate: func(args []reflect.Type) (reflect.Type, error) {
			if len(args) != 1 {
				return anyType, fmt.Errorf("invalid number of arguments (expected 1, got %d)", len(args))
			}
			switch kind(args[0]) {
			case reflect.Interface:
				return arrayType, nil
			case reflect.Map:
				return arrayType, nil
			}
			return anyType, fmt.Errorf("cannot get keys from %s", args[0])
		},
		Category:    CategoryMap,
		Description: "Returns an array containing the keys of the map.",
		Signature:   "keys(map)",
		Examples: []string{
			`keys({"name": "John", "age": 30}) == ["name", "age"]`,
		},
	},
	{
		Name: "values",
		Func: func(args ...any) (any, error) {
			if len(args) != 1 {
				return nil, fmt.Errorf("invalid number of arguments (expected 1, got %d)", len(args))
			}
			v := reflect.ValueOf(args[0])
			if v.Kind() != reflect.Map {
				return nil, fmt.Errorf("cannot get values from %s", v.Kind())
			}
			keys := v.MapKeys()
			out := make([]any, len(keys))
			for i, key := range keys {
				out[i] = v.MapIndex(key).Interface()
			}
			return out, nil
		},
		Validate: func(args []reflect.Type) (reflect.Type, error) {
			if len(args) != 1 {
				return anyType, fmt.Errorf("invalid number of arguments (expected 1, got %d)", len(args))
			}
			switch kind(args[0]) {
			case reflect.Interface:
				return arrayType, nil
			case reflect.Map:
				return arrayType, nil
			}
			return anyType, fmt.Errorf("cannot get values from %s", args[0])
		},
		Category:    CategoryMap,
		Description: "Returns an array containing the values of the map.",
		Signature:   "values(map)",
		Examples: []string{
			`values({"name": "John", "age": 30}) == ["John", 30]`,
		},
	},
	{
		Name: "toPairs",
		Func: func(args ...any) (any, error) {
			if len(args) != 1 {
				return nil, fmt.Errorf("invalid number of arguments (expected 1, got %d)", len(args))
			}
			v := reflect.ValueOf(args[0])
			if v.Kind() != reflect.Map {
				return nil, fmt.Errorf("cannot transform %s to pairs", v.Kind())
			}
			keys := v.MapKeys()
			out := make([][2]any, len(keys))
			for i, key := range keys {
				out[i] = [2]any{key.Interface(), v.MapIndex(key).Interface()}
			}
			return out, nil
		},
		Validate: func(args []reflect.Type) (reflect.Type, error) {
			if len(args) != 1 {
				return anyType, fmt.Errorf("invalid number of arguments (expected 1, got %d)", len(args))
			}
			switch kind(args[0]) {
			case reflect.Interface, reflect.Map:
				return arrayType, nil
			}
			return anyType, fmt.Errorf("cannot transform %s to pairs", args[0])
		},
		Category:    CategoryConversion,
		Description: "Converts a map to an array of key-value pairs.",
		Signature:   "toPairs(map)",
		Examples: []string{
			`toPairs({"name": "John", "age": 30}) == [["name", "John"], ["age", 30]]`,
		},
	},
	{
		Name: "fromPairs",
		Func: func(args ...any) (any, error) {
			if len(args) != 1 {
				return nil, fmt.Errorf("invalid number of arguments (expected 1, got %d)", len(args))
			}
			v := reflect.ValueOf(args[0])
			if v.Kind() != reflect.Slice && v.Kind() != reflect.Array {
				return nil, fmt.Errorf("cannot transform %s from pairs", v)
			}
			out := reflect.MakeMap(mapType)
			for i := 0; i < v.Len(); i++ {
				pair := deref.Value(v.Index(i))
				if pair.Kind() != reflect.Array && pair.Kind() != reflect.Slice {
					return nil, fmt.Errorf("invalid pair %v", pair)
				}
				if pair.Len() != 2 {
					return nil, fmt.Errorf("invalid pair length %v", pair)
				}
				key := pair.Index(0)
				value := pair.Index(1)
				out.SetMapIndex(key, value)
			}
			return out.Interface(), nil
		},
		Validate: func(args []reflect.Type) (reflect.Type, error) {
			if len(args) != 1 {
				return anyType, fmt.Errorf("invalid number of arguments (expected 1, got %d)", len(args))
			}
			switch kind(args[0]) {
			case reflect.Interface, reflect.Slice, reflect.Array:
				return mapType, nil
			}
			return anyType, fmt.Errorf("cannot transform %s from pairs", args[0])
		},
		Category:    CategoryConversion,
		Description: "Converts an array of key-value pairs to a map.",
		Signature:   "fromPairs(array)",
		Examples: []string{
			`fromPairs([["name", "John"], ["age", 30]])["name"] == "John"`,
		},
	},
	{
		Name: "reverse",
		Safe: func(args ...any) (any, uint, error) {
			if len(args) != 1 {
				return nil, 0, fmt.Errorf("invalid number of arguments (expected 1, got %d)", len(args))
			}

			v := reflect.ValueOf(args[0])
			if v.Kind() != reflect.Slice && v.Kind() != reflect.Array {
				return nil, 0, fmt.Errorf("cannot reverse %s", v.Kind())
			}

			size := v.Len()
			arr := make([]any, size)

			for i := 0; i < size; i++ {
				arr[i] = v.Index(size - i - 1).Interface()
			}

			return arr, uint(size), nil

		},
		Validate: func(args []reflect.Type) (reflect.Type, error) {
			if len(args) != 1 {
				return anyType, fmt.Errorf("invalid number of arguments (expected 1, got %d)", len(args))
			}
			switch kind(args[0]) {
			case reflect.Interface, reflect.Slice, reflect.Array:
				return arrayType, nil
			default:
				return anyType, fmt.Errorf("cannot reverse %s", args[0])
			}
		},
		Category:    CategoryArray,
		Description: "Return new reversed copy of the array.",
		Signature:   "reverse(array)",
		Examples: []string{
			`reverse([3, 1, 4]) == [4, 1, 3]`,
			`reverse(reverse([3, 1, 4])) == [3, 1, 4]`,
		},
	},

	{
		Name: "uniq",
		Func: func(args ...any) (any, error) {
			if len(args) != 1 {
				return nil, fmt.Errorf("invalid number of arguments (expected 1, got %d)", len(args))
			}

			v := reflect.ValueOf(args[0])
			if v.Kind() != reflect.Array && v.Kind() != reflect.Slice {
				return nil, fmt.Errorf("cannot uniq %s", v.Kind())
			}

			size := v.Len()
			ret := []any{}

			eq := func(i int) bool {
				for _, r := range ret {
					if runtime.Equal(v.Index(i).Interface(), r) {
						return true
					}
				}

				return false
			}

			for i := 0; i < size; i += 1 {
				if eq(i) {
					continue
				}

				ret = append(ret, v.Index(i).Interface())
			}

			return ret, nil
		},

		Validate: func(args []reflect.Type) (reflect.Type, error) {
			if len(args) != 1 {
				return anyType, fmt.Errorf("invalid number of arguments (expected 1, got %d)", len(args))
			}

			switch kind(args[0]) {
			case reflect.Interface, reflect.Slice, reflect.Array:
				return arrayType, nil
			default:
				return anyType, fmt.Errorf("cannot uniq %s", args[0])
			}
		},
		Category:    CategoryArray,
		Description: "Removes duplicates from an array.",
		Signature:   "uniq(array)",
		Examples: []string{
			`uniq([1, 2, 3, 2, 1]) == [1, 2, 3]`,
		},
	},

	{
		Name: "concat",
		Safe: func(args ...any) (any, uint, error) {
			if len(args) == 0 {
				return nil, 0, fmt.Errorf("invalid number of arguments (expected at least 1, got 0)")
			}

			var size uint
			var arr []any

			for _, arg := range args {
				v := reflect.ValueOf(arg)

				if v.Kind() != reflect.Slice && v.Kind() != reflect.Array {
					return nil, 0, fmt.Errorf("cannot concat %s", v.Kind())
				}

				size += uint(v.Len())

				for i := 0; i < v.Len(); i++ {
					item := v.Index(i)
					arr = append(arr, item.Interface())
				}
			}

			return arr, size, nil
		},
		Validate: func(args []reflect.Type) (reflect.Type, error) {
			if len(args) == 0 {
				return anyType, fmt.Errorf("invalid number of arguments (expected at least 1, got 0)")
			}

			for _, arg := range args {
				switch kind(arg) {
				case reflect.Interface, reflect.Slice, reflect.Array:
				default:
					return anyType, fmt.Errorf("cannot concat %s", arg)
				}
			}

			return arrayType, nil
		},
		Category:    CategoryArray,
		Description: "Concatenates two or more arrays.",
		Signature:   "concat(array1, array2[, ...])",
		Examples: []string{
			`concat([1, 2], [3, 4]) == [1, 2, 3, 4]`,
		},
	},
	{
		Name: "flatten",
		Safe: func(args ...any) (any, uint, error) {
			var size uint
			if len(args) != 1 {
				return nil, 0, fmt.Errorf("invalid number of arguments (expected 1, got %d)", len(args))
			}
			v := reflect.ValueOf(args[0])
			if v.Kind() != reflect.Array && v.Kind() != reflect.Slice {
				return nil, size, fmt.Errorf("cannot flatten %s", v.Kind())
			}
			ret, err := flatten(v, 0)
			if err != nil {
				return nil, 0, err
			}
			size = uint(len(ret))
			return ret, size, nil
		},
		Validate: func(args []reflect.Type) (reflect.Type, error) {
			if len(args) != 1 {
				return anyType, fmt.Errorf("invalid number of arguments (expected 1, got %d)", len(args))
			}

			for _, arg := range args {
				switch kind(arg) {
				case reflect.Interface, reflect.Slice, reflect.Array:
				default:
					return anyType, fmt.Errorf("cannot flatten %s", arg)
				}
			}

			return arrayType, nil
		},
		Category:    CategoryArray,
		Description: "Flattens given array into one-dimensional array.",
		Signature:   "flatten(array)",
		Examples: []string{
			`flatten([1, 2, [3, 4]]) == [1, 2, 3, 4]`,
		},
	},
	{
		Name: "sort",
		Safe: func(args ...any) (any, uint, error) {
			if len(args) != 1 && len(args) != 2 {
				return nil, 0, fmt.Errorf("invalid number of arguments (expected 1 or 2, got %d)", len(args))
			}

			var array []any

			switch in := args[0].(type) {
			case []any:
				array = make([]any, len(in))
				copy(array, in)
			case []int:
				array = make([]any, len(in))
				for i, v := range in {
					array[i] = v
				}
			case []float64:
				array = make([]any, len(in))
				for i, v := range in {
					array[i] = v
				}
			case []string:
				array = make([]any, len(in))
				for i, v := range in {
					array[i] = v
				}
			}

			var desc bool
			if len(args) == 2 {
				order, ok := args[1].(string)
				if !ok {
					return nil, 0, fmt.Errorf("sort order argument must be a string (got %T)", args[1])
				}
				switch order {
				case "asc":
					desc = false
				case "desc":
					desc = true
				default:
					return nil, 0, fmt.Errorf("invalid order %s, expected asc or desc", order)
				}
			}

			sortable := &runtime.Sort{
				Desc:  desc,
				Array: array,
			}
			sort.Sort(sortable)

			return sortable.Array, uint(len(array)), nil
		},
		Types: types(
			new(func([]any, string) []any),
			new(func([]int, string) []any),
			new(func([]float64, string) []any),
			new(func([]string, string) []any),

			new(func([]any) []any),
			new(func([]float64) []any),
			new(func([]string) []any),
			new(func([]int) []any),
		),
		Category: CategoryArray,
		Description: "Sorts an array in ascending order. Optional `order` argument can be used to specify the order of sorting: `asc`\n" +
			"or `desc`.",
		Signature: "sort(array[, order])",
		Examples: []string{
			`sort([3, 1, 4]) == [1, 3, 4]`,
			`sort([3, 1, 4], "desc") == [4, 3, 1]`,
		},
	},
	bitFunc(&Function{
		Name:        "bitand",
		Category:    CategoryBitwise,
		Description: "Returns the values resulting from the bitwise AND operation.",
		Signature:   "bitand(int, int)",
		Examples: []string{
			`bitand(0b1010, 0b1100) == 0b1000`,
		},
	}, func(x, y int) (any, error) {
		return x & y, nil
	}),
	bitFunc(&Function{
		Name:        "bitor",
		Category:    CategoryBitwise,
		Description: "Returns the values resulting from the bitwise OR operation.",
		Signature:   "bitor(int, int)",
		Examples: []string{
			`bitor(0b1010, 0b1100) == 0b1110`,
		},
	}, func(x, y int) (any, error) {
		return x | y, nil
	}),
	bitFunc(&Function{
		Name:        "bitxor",
		Category:    CategoryBitwise,
		Description: "Returns the values resulting from the bitwise XOR operation.",
		Signature:   "bitxor(int, int)",
		Examples: []string{
			`bitxor(0b1010, 0b1100) == 0b110`,
		},
	}, func(x, y int) (any, error) {
		return x ^ y, nil
	}),
	bitFunc(&Function{
		Name:        "bitnand",
		Category:    CategoryBitwise,
		Description: "Returns the values resulting from the bitwise AND NOT operation.",
		Signature:   "bitnand(int, int)",
		Examples: []string{
			`bitnand(0b1010, 0b1100) == 0b10`,
		},
	}, func(x, y int) (any, error) {
		return x &^ y, nil
	}),
	bitFunc(&Function{
		Name:        "bitshl",
		Category:    CategoryBitwise,
		Description: "Returns the values resulting from the Left Shift operation.",
		Signature:   "bitshl(int, int)",
		Examples: []string{
			`bitshl(0b101101, 2) == 0b10110100`,
		},
	}, func(x, y int) (any, error) {
		if y < 0 {
			return nil, fmt.Errorf("invalid operation: negative shift count %d (type int)", y)
		}
		return x << y, nil
	}),
	bitFunc(&Function{
		Name:        "bitshr",
		Category:    CategoryBitwise,
		Description: "Returns the values resulting from the Right Shift operation.",
		Signature:   "bitshr(int, int)",
		Examples: []string{
			`bitshr(0b101101, 2) == 0b1011`,
		},
	}, func(x, y int) (any, error) {
		if y < 0 {
			return nil, fmt.Errorf("invalid operation: negative shift count %d (type int)", y)
		}
		return x >> y, nil
	}),
	bitFunc(&Function{
		Name:        "bitushr",
		Category:    CategoryBitwise,
		Description: "Returns the values resulting from the unsigned Right Shift operation.",
		Signature:   "bitushr(int, int)",
		Examples: []string{
			`bitushr(-0b101, 2) == 4611686018427387902`,
		},
	}, func(x, y int) (any, error) {
		if y < 0 {
			return nil, fmt.Errorf("invalid operation: negative shift count %d (type int)", y)
		}
		return int(uint(x) >> y), nil
	}),
	{
		Name: "bitnot",
		Func: func(args ...any) (any, error) {
			if len(args) != 1 {
				return nil, fmt.Errorf("invalid number of arguments for bitnot (expected 1, got %d)", len(args))
			}
			x, err := toInt(args[0])
			if err != nil {
				return nil, fmt.Errorf("%v to call bitnot", err)
			}
			return ^x, nil
		},
		Types:       types(new(func(int) int)),
		Category:    CategoryBitwise,
		Description: "Returns the values resulting from the bitwise NOT operation.",
		Signature:   "bitnot(int)",
		Examples: []string{
			`bitnot(0b1010) == -0b1011`,
		},
	},
}
