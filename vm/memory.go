package vm

import (
	"fmt"

	"github.com/expr-lang/expr/builtin"
)

// This file contains the runtime memory allocator used to account for
// allocations whose size grows with the input.
//
// Accounting boundaries:
//
//   - Accounting state is held per VM instance. A VM must not be used to run
//     programs concurrently (the same applies to its Stack and Scopes), but a
//     compiled Program is immutable and may be shared between multiple VM
//     instances running concurrently. Each instance then accounts
//     independently, with no shared mutable state.
//   - Counters are reset to zero at the beginning of every VM.Run. Reusing a
//     VM instance never carries usage over from a previous run. Usage is not
//     accumulated across runs.
//   - The effective limit is resolved at the beginning of each run: an
//     explicitly set, non-zero VM.MemoryBudget takes precedence, otherwise the
//     limit carried by the compiled Program is used, and when that is not set
//     the conf.DefaultMemoryBudget applies.

// MemoryCategory identifies the kind of runtime allocation being accounted.
// The string values are part of the over-limit error message and must remain
// stable.
type MemoryCategory string

const (
	// MemoryRange accounts for range construction (OpRange).
	MemoryRange MemoryCategory = "range"
	// MemoryArray accounts for array construction (OpArray).
	MemoryArray MemoryCategory = "array"
	// MemoryMap accounts for map construction (OpMap) and other map-like
	// aggregate results.
	MemoryMap MemoryCategory = "map"
	// MemoryFunction accounts for values returned by function calls:
	// self-reported usage of safe functions, declared memory metrics, or the
	// default metric for all other call kinds.
	MemoryFunction MemoryCategory = "function"
	// MemorySort accounts for sortBy intermediate collection slices and the
	// final sort result. A sortBy over n items charges 2*n for the two
	// intermediate slices reserved at OpCreate and keeps the historical n
	// units at sort finalization (3*n in total); the overlap is intentional so
	// that the legacy finalization accounting stays byte-for-byte equivalent.
	MemorySort MemoryCategory = "sort"
	// MemoryString accounts for string concatenation (OpAdd).
	MemoryString MemoryCategory = "string"
	// MemorySlice accounts for slicing operations (OpSlice).
	MemorySlice MemoryCategory = "slice"
	// MemoryGroup accounts for items appended to groupBy aggregates.
	MemoryGroup MemoryCategory = "group"
)

// memoryCategoryCount is the number of accounted categories. It must stay in
// sync with memoryCategoryOrder and the exported category constants.
const memoryCategoryCount = 8

// memoryCategoryOrder fixes the ordinal of each category in the allocator's
// inline counters; the order is not part of any public API.
var memoryCategoryOrder = [memoryCategoryCount]MemoryCategory{
	MemoryRange,
	MemoryArray,
	MemoryMap,
	MemoryFunction,
	MemorySort,
	MemoryString,
	MemorySlice,
	MemoryGroup,
}

// memoryCategoryIndex is an immutable, package-wide lookup table shared by all
// runs (read-only after initialization, so it is safe for concurrent VMs).
var memoryCategoryIndex = func() map[MemoryCategory]int {
	index := make(map[MemoryCategory]int, memoryCategoryCount)
	for i, category := range memoryCategoryOrder {
		index[category] = i
	}
	return index
}()

// memoryAllocator is the single runtime accounting channel. Every
// input-size-dependent allocation requests units through grow. Per-category
// counters are stored in an inline fixed-size array so that even a one-shot VM
// (such as the one created by package-level Run) allocates nothing per run.
type memoryAllocator struct {
	budget     uint
	used       uint
	categories [memoryCategoryCount]uint
}

// reset zeroes all counters and sets the effective limit. It is called once at
// the beginning of every run and performs no heap allocations.
func (a *memoryAllocator) reset(budget uint) {
	a.budget = budget
	a.used = 0
	for i := range a.categories {
		a.categories[i] = 0
	}
}

// grow requests size units for an allocation of the given category. It keeps
// the per-category and total counters consistent and panics when the resulting
// total would reach or exceed the budget (matching the historical >=
// semantics). A rejected allocation is not recorded, as the run aborts and
// counters are reset on the next run anyway.
func (a *memoryAllocator) grow(category MemoryCategory, size uint) {
	index := memoryCategoryIndex[category]
	categoryUsed := a.categories[index] + size
	total := a.used + size
	if total >= a.budget {
		panic(fmt.Sprintf(
			"memory budget exceeded: %s allocation %d exceeds limit %d (category used %d, total used %d)",
			category, size, a.budget, categoryUsed, total,
		))
	}
	a.categories[index] = categoryUsed
	a.used = total
}

// measureMemory reports the accounted size of a runtime value using the same
// default metric as functions without a declared memory metric.
func measureMemory(value any) uint {
	return builtin.DefaultMemoryFunc(nil, value)
}

// chargeFunctionResult applies the declared metric (or the default result
// metric) to a function call result and requests the units from the allocator.
func (vm *VM) chargeFunctionResult(memory builtin.MemoryFunc, args []any, result any) {
	if memory == nil {
		memory = builtin.DefaultMemoryFunc
	}
	vm.mem.grow(MemoryFunction, memory(args, result))
}

// MemoryUsage is a read-only snapshot of the allocator counters taken after
// (or between) runs.
type MemoryUsage struct {
	// Limit is the effective budget of the snapshot run; zero if the VM has
	// never run.
	Limit uint
	// Used is the total accounted usage.
	Used uint
	// Categories holds cumulative usage keyed by category.
	Categories map[MemoryCategory]uint
}

// MemoryUsage returns a copy of the current allocator counters. It does not
// expose mutable state of the VM. The category map is materialized only for
// the snapshot, so the execution hot path stays allocation-free.
func (vm *VM) MemoryUsage() MemoryUsage {
	categories := make(map[MemoryCategory]uint, memoryCategoryCount)
	for i, category := range memoryCategoryOrder {
		if used := vm.mem.categories[i]; used != 0 {
			categories[category] = used
		}
	}
	return MemoryUsage{
		Limit:      vm.mem.budget,
		Used:       vm.mem.used,
		Categories: categories,
	}
}
