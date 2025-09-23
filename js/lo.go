package js

import (
	"context"
	"math/rand"
	"strconv"
	"time"

	"github.com/dop251/goja"
	"github.com/samber/lo"
)

// LoBinds добавляет все функции библиотеки lo в JavaScript runtime
func LoBinds(vm *goja.Runtime) {
	obj := vm.NewObject()
	vm.Set("$lo", obj)

	// 1. Slice functions (функции для работы со слайсами)
	sliceObj := vm.NewObject()
	obj.Set("slice", sliceObj)

	// Filter
	sliceObj.Set("filter", func(slice []any, predicate func(any, int) bool) []any {
		return lo.Filter(slice, predicate)
	})

	// Map
	sliceObj.Set("map", func(slice []any, mapper func(any, int) any) []any {
		return lo.Map(slice, mapper)
	})

	// UniqMap
	sliceObj.Set("uniqMap", func(slice []any, mapper func(any, int) any) []any {
		return lo.UniqMap(slice, mapper)
	})

	// FilterMap
	sliceObj.Set("filterMap", func(slice []any, fn func(any, int) (any, bool)) []any {
		return lo.FilterMap(slice, fn)
	})

	// FlatMap
	sliceObj.Set("flatMap", func(slice []any, mapper func(any, int) []any) []any {
		return lo.FlatMap(slice, mapper)
	})

	// Reduce
	sliceObj.Set("reduce", func(slice []any, reducer func(any, any, int) any, initial any) any {
		return lo.Reduce(slice, reducer, initial)
	})

	// ReduceRight
	sliceObj.Set("reduceRight", func(slice []any, reducer func(any, any, int) any, initial any) any {
		return lo.ReduceRight(slice, reducer, initial)
	})

	// ForEach
	sliceObj.Set("forEach", func(slice []any, fn func(any, int)) {
		lo.ForEach(slice, fn)
	})

	// ForEachWhile
	sliceObj.Set("forEachWhile", func(slice []any, fn func(any, int) bool) {
		lo.ForEachWhile(slice, fn)
	})

	// Times
	sliceObj.Set("times", func(count int, fn func(int) any) []any {
		return lo.Times(count, fn)
	})

	// Uniq
	sliceObj.Set("uniq", func(slice []any) []any {
		return lo.Uniq(slice)
	})

	// UniqBy
	sliceObj.Set("uniqBy", func(slice []any, iteratee func(any) any) []any {
		return lo.UniqBy(slice, iteratee)
	})

	// GroupBy
	sliceObj.Set("groupBy", func(slice []any, iteratee func(any) any) map[string][]any {
		result := make(map[string][]any)
		grouped := lo.GroupBy(slice, iteratee)
		for k, v := range grouped {
			result[toString(k)] = v
		}
		return result
	})

	// Chunk
	sliceObj.Set("chunk", func(slice []any, size int) [][]any {
		return lo.Chunk(slice, size)
	})

	// PartitionBy
	sliceObj.Set("partitionBy", func(slice []any, iteratee func(any) any) [][]any {
		return lo.PartitionBy(slice, iteratee)
	})

	// Flatten
	sliceObj.Set("flatten", func(slice [][]any) []any {
		return lo.Flatten(slice)
	})

	// Interleave
	sliceObj.Set("interleave", func(slices ...[]any) []any {
		return lo.Interleave(slices...)
	})

	// Shuffle
	sliceObj.Set("shuffle", func(slice []any) []any {
		return lo.Shuffle(slice)
	})

	// Reverse
	sliceObj.Set("reverse", func(slice []any) []any {
		return lo.Reverse(slice)
	})

	// TODO: Fill
	// sliceObj.Set("fill", func(slice []any, value any) []any {
	// 	return lo.Fill(slice, value)
	// })

	// TODO: Repeat
	// sliceObj.Set("repeat", func(count int, value any) []any {
	// 	return lo.Repeat(count, value)
	// })

	// RepeatBy
	sliceObj.Set("repeatBy", func(count int, fn func(int) any) []any {
		return lo.RepeatBy(count, fn)
	})

	// KeyBy
	sliceObj.Set("keyBy", func(slice []any, iteratee func(any) any) map[string]any {
		result := make(map[string]any)
		keyed := lo.KeyBy(slice, iteratee)
		for k, v := range keyed {
			result[toString(k)] = v
		}
		return result
	})

	// SliceToMap
	sliceObj.Set("sliceToMap", func(slice []any, transform func(any) (any, any)) map[string]any {
		result := make(map[string]any)
		mapped := lo.SliceToMap(slice, transform)
		for k, v := range mapped {
			result[toString(k)] = v
		}
		return result
	})

	// Drop
	sliceObj.Set("drop", func(slice []any, n int) []any {
		return lo.Drop(slice, n)
	})

	// DropRight
	sliceObj.Set("dropRight", func(slice []any, n int) []any {
		return lo.DropRight(slice, n)
	})

	// DropWhile
	sliceObj.Set("dropWhile", func(slice []any, predicate func(any) bool) []any {
		return lo.DropWhile(slice, predicate)
	})

	// DropRightWhile
	sliceObj.Set("dropRightWhile", func(slice []any, predicate func(any) bool) []any {
		return lo.DropRightWhile(slice, predicate)
	})

	// Reject
	sliceObj.Set("reject", func(slice []any, predicate func(any, int) bool) []any {
		return lo.Reject(slice, predicate)
	})

	// Count
	sliceObj.Set("count", func(slice []any, value any) int {
		return lo.Count(slice, value)
	})

	// CountBy
	sliceObj.Set("countBy", func(slice []any, predicate func(any) bool) int {
		return lo.CountBy(slice, predicate)
	})

	// CountValues
	sliceObj.Set("countValues", func(slice []any) map[string]int {
		result := make(map[string]int)
		counted := lo.CountValues(slice)
		for k, v := range counted {
			result[toString(k)] = v
		}
		return result
	})

	// Subset
	sliceObj.Set("subset", func(slice []any, offset, length int) []any {
		return lo.Subset(slice, offset, uint(length))
	})

	// Slice
	sliceObj.Set("slice", func(slice []any, start, end int) []any {
		return lo.Slice(slice, start, end)
	})

	// Replace
	sliceObj.Set("replace", func(slice []any, old, new any, n int) []any {
		return lo.Replace(slice, old, new, n)
	})

	// ReplaceAll
	sliceObj.Set("replaceAll", func(slice []any, old, new any) []any {
		return lo.ReplaceAll(slice, old, new)
	})

	// Compact
	sliceObj.Set("compact", func(slice []any) []any {
		return lo.Compact(slice)
	})

	// TODO: IsSorted
	// sliceObj.Set("isSorted", func(slice []any) bool {
	// 	return lo.IsSorted(slice)
	// })

	// 2. Map functions (функции для работы с картами)
	mapObj := vm.NewObject()
	obj.Set("map", mapObj)

	// Keys
	mapObj.Set("keys", func(m map[string]any) []string {
		return lo.Keys(m)
	})

	// Values
	mapObj.Set("values", func(m map[string]any) []any {
		return lo.Values(m)
	})

	// HasKey
	mapObj.Set("hasKey", func(m map[string]any, key string) bool {
		return lo.HasKey(m, key)
	})

	// ValueOr
	mapObj.Set("valueOr", func(m map[string]any, key string, fallback any) any {
		return lo.ValueOr(m, key, fallback)
	})

	// PickBy
	mapObj.Set("pickBy", func(m map[string]any, predicate func(string, any) bool) map[string]any {
		return lo.PickBy(m, predicate)
	})

	// PickByKeys
	mapObj.Set("pickByKeys", func(m map[string]any, keys []string) map[string]any {
		return lo.PickByKeys(m, keys)
	})

	// PickByValues
	mapObj.Set("pickByValues", func(m map[string]any, values []any) map[string]any {
		return lo.PickByValues(m, values)
	})

	// OmitBy
	mapObj.Set("omitBy", func(m map[string]any, predicate func(string, any) bool) map[string]any {
		return lo.OmitBy(m, predicate)
	})

	// OmitByKeys
	mapObj.Set("omitByKeys", func(m map[string]any, keys []string) map[string]any {
		return lo.OmitByKeys(m, keys)
	})

	// OmitByValues
	mapObj.Set("omitByValues", func(m map[string]any, values []any) map[string]any {
		return lo.OmitByValues(m, values)
	})

	// Entries
	mapObj.Set("entries", func(m map[string]any) []map[string]any {
		entries := lo.Entries(m)
		result := make([]map[string]any, len(entries))
		for i, entry := range entries {
			result[i] = map[string]any{
				"key":   entry.Key,
				"value": entry.Value,
			}
		}
		return result
	})

	// FromEntries
	mapObj.Set("fromEntries", func(entries []map[string]any) map[string]any {
		result := make(map[string]any)
		for _, entry := range entries {
			if key, ok := entry["key"].(string); ok {
				result[key] = entry["value"]
			}
		}
		return result
	})

	// Invert
	mapObj.Set("invert", func(m map[string]any) map[string]string {
		result := make(map[string]string)
		inverted := lo.Invert(m)
		for k, v := range inverted {
			result[toString(k)] = v
		}
		return result
	})

	// Assign
	mapObj.Set("assign", func(maps ...map[string]any) map[string]any {
		return lo.Assign(maps...)
	})

	// MapKeys
	mapObj.Set("mapKeys", func(m map[string]any, mapper func(any, string) string) map[string]any {
		return lo.MapKeys(m, mapper)
	})

	// MapValues
	mapObj.Set("mapValues", func(m map[string]any, mapper func(any, string) any) map[string]any {
		return lo.MapValues(m, mapper)
	})

	// MapToSlice
	mapObj.Set("mapToSlice", func(m map[string]any, mapper func(string, any) any) []any {
		return lo.MapToSlice(m, mapper)
	})

	// 3. Math functions (математические функции)
	mathObj := vm.NewObject()
	obj.Set("math", mathObj)

	// Range
	mathObj.Set("range", func(size int) []int {
		return lo.Range(size)
	})

	// RangeFrom
	mathObj.Set("rangeFrom", func(start, size int) []int {
		return lo.RangeFrom(start, size)
	})

	// RangeWithSteps
	mathObj.Set("rangeWithSteps", func(start, end, step int) []int {
		return lo.RangeWithSteps(start, end, step)
	})

	// Clamp
	mathObj.Set("clamp", func(value, min, max float64) float64 {
		return lo.Clamp(value, min, max)
	})

	// Sum
	mathObj.Set("sum", func(values []float64) float64 {
		return lo.Sum(values)
	})

	// SumBy
	mathObj.Set("sumBy", func(slice []any, mapper func(any) float64) float64 {
		return lo.SumBy(slice, mapper)
	})

	// Product
	mathObj.Set("product", func(values []float64) float64 {
		return lo.Product(values)
	})

	// ProductBy
	mathObj.Set("productBy", func(slice []any, mapper func(any) float64) float64 {
		return lo.ProductBy(slice, mapper)
	})

	// Mean
	mathObj.Set("mean", func(values []float64) float64 {
		return lo.Mean(values)
	})

	// MeanBy
	mathObj.Set("meanBy", func(slice []any, mapper func(any) float64) float64 {
		return lo.MeanBy(slice, mapper)
	})

	// 4. String functions (функции для работы со строками)
	stringObj := vm.NewObject()
	obj.Set("string", stringObj)

	// RandomString
	stringObj.Set("randomString", func(size int, charset string) string {
		runes := []rune(charset)
		return lo.RandomString(size, runes)
	})

	// Substring
	stringObj.Set("substring", func(str string, offset, length int) string {
		return lo.Substring(str, offset, uint(length))
	})

	// ChunkString
	stringObj.Set("chunkString", func(str string, size int) []string {
		return lo.ChunkString(str, size)
	})

	// RuneLength
	stringObj.Set("runeLength", func(str string) int {
		return lo.RuneLength(str)
	})

	// PascalCase
	stringObj.Set("pascalCase", func(str string) string {
		return lo.PascalCase(str)
	})

	// CamelCase
	stringObj.Set("camelCase", func(str string) string {
		return lo.CamelCase(str)
	})

	// KebabCase
	stringObj.Set("kebabCase", func(str string) string {
		return lo.KebabCase(str)
	})

	// SnakeCase
	stringObj.Set("snakeCase", func(str string) string {
		return lo.SnakeCase(str)
	})

	// Words
	stringObj.Set("words", func(str string) []string {
		return lo.Words(str)
	})

	// Capitalize
	stringObj.Set("capitalize", func(str string) string {
		return lo.Capitalize(str)
	})

	// Ellipsis
	stringObj.Set("ellipsis", func(str string, length int) string {
		return lo.Ellipsis(str, length)
	})

	// 5. Search functions (функции поиска)
	searchObj := vm.NewObject()
	obj.Set("search", searchObj)

	// IndexOf
	searchObj.Set("indexOf", func(slice []any, value any) int {
		return lo.IndexOf(slice, value)
	})

	// LastIndexOf
	searchObj.Set("lastIndexOf", func(slice []any, value any) int {
		return lo.LastIndexOf(slice, value)
	})

	// Find
	searchObj.Set("find", func(slice []any, predicate func(any) bool) map[string]any {
		value, found := lo.Find(slice, predicate)
		return map[string]any{
			"value": value,
			"found": found,
		}
	})

	// FindIndexOf
	searchObj.Set("findIndexOf", func(slice []any, predicate func(any) bool) map[string]any {
		value, index, found := lo.FindIndexOf(slice, predicate)
		return map[string]any{
			"value": value,
			"index": index,
			"found": found,
		}
	})

	// FindOrElse
	searchObj.Set("findOrElse", func(slice []any, fallback any, predicate func(any) bool) any {
		return lo.FindOrElse(slice, fallback, predicate)
	})

	// FindKey
	searchObj.Set("findKey", func(m map[string]any, value any) map[string]any {
		key, found := lo.FindKey(m, value)
		return map[string]any{
			"key":   key,
			"found": found,
		}
	})

	// FindKeyBy
	searchObj.Set("findKeyBy", func(m map[string]any, predicate func(string, any) bool) map[string]any {
		key, found := lo.FindKeyBy(m, predicate)
		return map[string]any{
			"key":   key,
			"found": found,
		}
	})

	// FindUniques
	searchObj.Set("findUniques", func(slice []any) []any {
		return lo.FindUniques(slice)
	})

	// FindDuplicates
	searchObj.Set("findDuplicates", func(slice []any) []any {
		return lo.FindDuplicates(slice)
	})

	// TODO: Min
	// searchObj.Set("min", func(slice []any) any {
	// 	return lo.Min(slice)
	// })

	// TODO: Max
	// searchObj.Set("max", func(slice []any) any {
	// 	return lo.Max(slice)
	// })

	// MinBy
	searchObj.Set("minBy", func(slice []any, comparator func(any, any) bool) any {
		return lo.MinBy(slice, comparator)
	})

	// MaxBy
	searchObj.Set("maxBy", func(slice []any, comparator func(any, any) bool) any {
		return lo.MaxBy(slice, comparator)
	})

	// First
	searchObj.Set("first", func(slice []any) map[string]any {
		value, found := lo.First(slice)
		return map[string]any{
			"value": value,
			"found": found,
		}
	})

	// Last
	searchObj.Set("last", func(slice []any) map[string]any {
		value, found := lo.Last(slice)
		return map[string]any{
			"value": value,
			"found": found,
		}
	})

	// FirstOr
	searchObj.Set("firstOr", func(slice []any, fallback any) any {
		return lo.FirstOr(slice, fallback)
	})

	// LastOr
	searchObj.Set("lastOr", func(slice []any, fallback any) any {
		return lo.LastOr(slice, fallback)
	})

	// Nth
	searchObj.Set("nth", func(slice []any, index int) map[string]any {
		value, err := lo.Nth(slice, index)
		return map[string]any{
			"value": value,
			"error": err,
		}
	})

	// NthOr
	searchObj.Set("nthOr", func(slice []any, index int, fallback any) any {
		return lo.NthOr(slice, index, fallback)
	})

	// Sample
	searchObj.Set("sample", func(slice []any) any {
		return lo.Sample(slice)
	})

	// Samples
	searchObj.Set("samples", func(slice []any, count int) []any {
		return lo.Samples(slice, count)
	})

	// 6. Intersection functions (функции пересечений)
	intersectionObj := vm.NewObject()
	obj.Set("intersection", intersectionObj)

	// Contains
	intersectionObj.Set("contains", func(slice []any, value any) bool {
		return lo.Contains(slice, value)
	})

	// ContainsBy
	intersectionObj.Set("containsBy", func(slice []any, predicate func(any) bool) bool {
		return lo.ContainsBy(slice, predicate)
	})

	// Every
	intersectionObj.Set("every", func(slice, subset []any) bool {
		return lo.Every(slice, subset)
	})

	// EveryBy
	intersectionObj.Set("everyBy", func(slice []any, predicate func(any) bool) bool {
		return lo.EveryBy(slice, predicate)
	})

	// Some
	intersectionObj.Set("some", func(slice, subset []any) bool {
		return lo.Some(slice, subset)
	})

	// SomeBy
	intersectionObj.Set("someBy", func(slice []any, predicate func(any) bool) bool {
		return lo.SomeBy(slice, predicate)
	})

	// None
	intersectionObj.Set("none", func(slice, subset []any) bool {
		return lo.None(slice, subset)
	})

	// NoneBy
	intersectionObj.Set("noneBy", func(slice []any, predicate func(any) bool) bool {
		return lo.NoneBy(slice, predicate)
	})

	// Intersect
	intersectionObj.Set("intersect", func(slice1, slice2 []any) []any {
		return lo.Intersect(slice1, slice2)
	})

	// Difference
	intersectionObj.Set("difference", func(slice1, slice2 []any) map[string][]any {
		left, right := lo.Difference(slice1, slice2)
		return map[string][]any{
			"left":  left,
			"right": right,
		}
	})

	// Union
	intersectionObj.Set("union", func(slices ...[]any) []any {
		return lo.Union(slices...)
	})

	// Without
	intersectionObj.Set("without", func(slice []any, values ...any) []any {
		return lo.Without(slice, values...)
	})

	// WithoutEmpty
	intersectionObj.Set("withoutEmpty", func(slice []any) []any {
		return lo.WithoutEmpty(slice)
	})

	// 7. Conditional functions (условные функции)
	conditionalObj := vm.NewObject()
	obj.Set("conditional", conditionalObj)

	// Ternary
	conditionalObj.Set("ternary", func(condition bool, ifTrue, ifFalse any) any {
		return lo.Ternary(condition, ifTrue, ifFalse)
	})

	// TernaryF
	conditionalObj.Set("ternaryF", func(condition bool, ifTrue, ifFalse func() any) any {
		return lo.TernaryF(condition, ifTrue, ifFalse)
	})

	// 8. Type manipulation functions (функции манипуляции типов)
	typeObj := vm.NewObject()
	obj.Set("type", typeObj)

	// IsNil
	typeObj.Set("isNil", func(value any) bool {
		return lo.IsNil(value)
	})

	// IsNotNil
	typeObj.Set("isNotNil", func(value any) bool {
		return lo.IsNotNil(value)
	})

	// IsEmpty
	typeObj.Set("isEmpty", func(value any) bool {
		return lo.IsEmpty(value)
	})

	// IsNotEmpty
	typeObj.Set("isNotEmpty", func(value any) bool {
		return lo.IsNotEmpty(value)
	})

	// Coalesce
	typeObj.Set("coalesce", func(values ...any) map[string]any {
		value, ok := lo.Coalesce(values...)
		return map[string]any{
			"value": value,
			"ok":    ok,
		}
	})

	// CoalesceOrEmpty
	typeObj.Set("coalesceOrEmpty", func(values ...any) any {
		return lo.CoalesceOrEmpty(values...)
	})

	// 9. Error handling functions (функции обработки ошибок)
	errorObj := vm.NewObject()
	obj.Set("error", errorObj)

	// Try
	errorObj.Set("try", func(fn func() error) bool {
		return lo.Try(fn)
	})

	// TryOr
	errorObj.Set("tryOr", func(fn func() (any, error), fallback any) map[string]any {
		value, ok := lo.TryOr(fn, fallback)
		return map[string]any{
			"value": value,
			"ok":    ok,
		}
	})

	// TODO: TryCatch
	// errorObj.Set("tryCatch", func(fn func() error, catch func()) bool {
	// 	return lo.TryCatch(fn, catch)
	// })

	// 10. Concurrency functions (функции конкурентности)
	concurrencyObj := vm.NewObject()
	obj.Set("concurrency", concurrencyObj)

	// Attempt
	concurrencyObj.Set("attempt", func(maxAttempts int, fn func(int) error) map[string]any {
		iterations, err := lo.Attempt(maxAttempts, fn)
		return map[string]any{
			"iterations": iterations,
			"error":      err,
		}
	})

	// AttemptWithDelay
	concurrencyObj.Set("attemptWithDelay", func(maxAttempts int, delay int64, fn func(int, time.Duration) error) map[string]any {
		iterations, duration, err := lo.AttemptWithDelay(maxAttempts, time.Duration(delay)*time.Millisecond, fn)
		return map[string]any{
			"iterations": iterations,
			"duration":   duration.Milliseconds(),
			"error":      err,
		}
	})

	// WaitFor
	concurrencyObj.Set("waitFor", func(condition func(int) bool, maxDuration, interval int64) map[string]any {
		iterations, duration, ok := lo.WaitFor(condition, time.Duration(maxDuration)*time.Millisecond, time.Duration(interval)*time.Millisecond)
		return map[string]any{
			"iterations": iterations,
			"duration":   duration.Milliseconds(),
			"ok":         ok,
		}
	})

	// WaitForWithContext
	concurrencyObj.Set("waitForWithContext", func(ctx context.Context, condition func(context.Context, int) bool, maxDuration, interval int64) map[string]any {
		iterations, duration, ok := lo.WaitForWithContext(ctx, condition, time.Duration(maxDuration)*time.Millisecond, time.Duration(interval)*time.Millisecond)
		return map[string]any{
			"iterations": iterations,
			"duration":   duration.Milliseconds(),
			"ok":         ok,
		}
	})

	// 13. Constants (константы)
	constantsObj := vm.NewObject()
	obj.Set("constants", constantsObj)

	constantsObj.Set("lettersCharset", lo.LettersCharset)
	constantsObj.Set("numbersCharset", lo.NumbersCharset)
	constantsObj.Set("alphanumericCharset", lo.AlphanumericCharset)

	// 14. Utility functions (вспомогательные функции)
	utilObj := vm.NewObject()
	obj.Set("util", utilObj)

	// Random
	utilObj.Set("random", func(min, max int) int {
		return rand.Intn(max-min) + min
	})

	// Sleep
	utilObj.Set("sleep", func(ms int64) {
		time.Sleep(time.Duration(ms) * time.Millisecond)
	})

	// Now
	utilObj.Set("now", func() int64 {
		return time.Now().UnixMilli()
	})
}

// Вспомогательная функция для преобразования в строку
func toString(v any) string {
	switch val := v.(type) {
	case string:
		return val
	case int:
		return strconv.Itoa(val)
	case int64:
		return strconv.FormatInt(val, 10)
	case float64:
		return strconv.FormatFloat(val, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(val)
	default:
		return ""
	}
}
