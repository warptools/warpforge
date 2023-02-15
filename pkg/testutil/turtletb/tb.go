// TurtleTB creates an implementation of TB that is more inspectable.
// May be used where failures are expected.
// Why "turtletb"? Because "tb" is too short for a package name and is a commonly used variable string. I couldn't think of anything good. It's turtles all the way down.
package turtletb

import (
	"context"
	"fmt"
	"runtime"
	"testing"
)

var _ testing.TB = &TB{}

// TB is a very simplified implementation of testing.TB
// Some functions rely on a "parent" implementation.
// Particularly "Helper" must be implemented by the parent.
//
// Use "Start" to run tests within a goroutine that will exit on failure.
type TB struct {
	testing.TB
	ctx     context.Context
	failed  bool
	skipped bool
	Records []Record // Stores logs for later inspection. Any calls to Log/Error/Skip which takes inputs to log will end up in here as well as logged to the parent TB.
}

type RecordKind int

const (
	RK_Log RecordKind = 1 << iota
	RK_Error
	RK_Skip
)

type Record struct {
	Kind  RecordKind // RecordKind is a bitfield which allows easy filtering
	Value string
}

// Start will create a goroutine which runs the function
func (tb *TB) Start(ctx context.Context, f func(testing.TB)) context.Context {
	if tb.ctx != nil {
		panic("tb: TB instance already started")
	}
	ctx, cancel := context.WithCancel(ctx)
	go func() {
		defer cancel()
		f(tb)
	}()
	return ctx
}

func (tb *TB) record(kind RecordKind, args ...any) {
	r := Record{Kind: kind, Value: fmt.Sprint(args...)}
	tb.Records = append(tb.Records, r)
	if tb.TB != nil {
		tb.TB.Helper()
		tb.TB.Log(args...)
	}
}

func (tb *TB) recordf(kind RecordKind, format string, args ...any) {
	r := Record{Kind: kind, Value: fmt.Sprintf(format, args...)}
	tb.Records = append(tb.Records, r)
	if tb.TB != nil {
		tb.TB.Helper()
		tb.TB.Logf(format, args...)
	}
}

// Cleanup implements testing.TB
func (tb *TB) Cleanup(f func()) {
	if tb.TB != nil {
		tb.TB.Helper()
	}
	tb.TB.Cleanup(f)
}

// Error implements testing.TB
func (tb *TB) Error(args ...any) {
	if tb.TB != nil {
		tb.TB.Helper()
	}
	tb.record(RK_Error, args...)
	tb.Fail()
}

// Errorf implements testing.TB
func (tb *TB) Errorf(format string, args ...any) {
	if tb.TB != nil {
		tb.TB.Helper()
	}
	tb.recordf(RK_Error, format, args...)
	tb.Fail()
}

// Fail implements testing.TB
func (tb *TB) Fail() {
	if tb.TB != nil {
		tb.TB.Helper()
	}
	tb.failed = true
}

// FailNow implements testing.TB
func (tb *TB) FailNow() {
	if tb.TB != nil {
		tb.TB.Helper()
	}
	tb.Fail()
	runtime.Goexit()
}

// Failed implements testing.TB
func (tb *TB) Failed() bool {
	return tb.failed
}

// Fatal implements testing.TB
func (tb *TB) Fatal(args ...any) {
	if tb.TB != nil {
		tb.TB.Helper()
	}
	tb.Error(args...)
	tb.FailNow()
}

// Fatalf implements testing.TB
func (tb *TB) Fatalf(format string, args ...any) {
	if tb.TB != nil {
		tb.TB.Helper()
	}
	tb.Errorf(format, args...)
	tb.FailNow()
}

// Log implements testing.TB
func (tb *TB) Log(args ...any) {
	if tb.TB != nil {
		tb.TB.Helper()
	}
	tb.record(RK_Log, args...)
}

// Logf implements testing.TB
func (tb *TB) Logf(format string, args ...any) {
	if tb.TB != nil {
		tb.TB.Helper()
	}
	tb.recordf(RK_Log, format, args...)
}

// Name implements testing.TB
func (tb *TB) Name() string {
	return tb.TB.Name()
}

// Setenv implements testing.TB
func (tb *TB) Setenv(key string, value string) {
	tb.TB.Setenv(key, value)
}

// Skip implements testing.TB
func (tb *TB) Skip(args ...any) {
	if tb.TB != nil {
		tb.TB.Helper()
	}
	tb.record(RK_Skip, args...)
	tb.SkipNow()
}

// SkipNow implements testing.TB
func (tb *TB) SkipNow() {
	if tb.TB != nil {
		tb.TB.Helper()
	}
	tb.skipped = true
	runtime.Goexit()
}

// Skipf implements testing.TB
func (tb *TB) Skipf(format string, args ...any) {
	if tb.TB != nil {
		tb.TB.Helper()
	}
	tb.recordf(RK_Skip, format, args...)
	tb.SkipNow()
}

// Skipped implements testing.TB
func (tb *TB) Skipped() bool {
	return tb.skipped
}

// TempDir implements testing.TB
func (tb *TB) TempDir() string {
	return tb.TB.TempDir()
}
