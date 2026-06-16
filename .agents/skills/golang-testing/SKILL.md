---
name: golang-testing
description: "Production-ready testing practices in Go: table-driven tests, named subtests, test isolation, mock generation/design, integration tests isolation using build tags, and avoiding flakiness. Apply when writing, reviewing, or debugging Go tests."
user-invocable: true
license: MIT
compatibility: Designed for Go testing suites.
---

# Go Testing Best Practices

This skill outlines guidelines for writing maintainable, fast, and deterministic tests in Go projects.

## 1. Table-Driven Tests (Idiomatic Go)

Always use table-driven tests for multiple inputs/scenarios. Each scenario MUST have a descriptive `name` field, which is passed to `t.Run`.

```go
func TestAdd(t *testing.T) {
    tests := []struct {
        name     string
        a, b     int
        expected int
    }{
        {
            name:     "positive numbers",
            a:        2,
            b:        3,
            expected: 5,
        },
        {
            name:     "negative and positive",
            a:        -1,
            b:        1,
            expected: 0,
        },
        {
            name:     "both negative",
            a:        -2,
            b:        -3,
            expected: -5,
        },
    }

    for _, tt := range tests {
        // Capture range variable to avoid concurrency issues in loops
        tt := tt
        t.Run(tt.name, func(t *testing.T) {
            got := Add(tt.a, tt.b)
            if got != tt.expected {
                t.Errorf("Add(%d, %d) = %d; want %d", tt.a, tt.b, got, tt.expected)
            }
        })
    }
}
```

---

## 2. Test Parallelism

- Independent unit tests SHOULD run in parallel.
- Call `t.Parallel()` inside the main test and also inside individual `t.Run` subtests.
- *Caution*: Always capture the range variable (e.g., `tt := tt`) at the top of the loop before running subtests in parallel to prevent goroutines from referring to the last item in the slice.

---

## 3. Mocking & Dependencies

1. **Mock Interfaces, Not Structs**:
   - Write functions to accept interfaces, enabling easy mocking in tests.
2. **Minimalist Mocks**:
   - Prefer hand-written mock structs implementing only the needed methods, or use code generation tools like `mockery` or `go-mock` (avoid manual complex mocks that break on interface changes).
3. **Use Test Cleanups**:
   - Use `t.Cleanup(func() { ... })` instead of `defer` inside subtests for cleaning up test databases, directories, or mocks. `t.Cleanup` runs even if the test panics or fails early.

---

## 4. Integration vs. Unit Tests

1. **Keep Unit Tests Fast**:
   - Unit tests should execute in < 10ms and run without network or filesystem dependencies.
2. **Isolate Integration Tests**:
   - Separate integration tests (which rely on databases, AWS, or local networks) using build tags:
     ```go
     //go:build integration
     
     package mypackage_test
     ```
   - Run unit tests with standard `go test ./...`.
   - Run integration tests explicitly with `go test -tags=integration ./...`.

---

## 5. Avoiding Test Flakiness

1. **No Shared State**:
   - Ensure tests are completely independent and don't share global mutable state.
2. **Order Independence**:
   - Tests must run successfully in any order. Verify with:
     ```bash
     go test -shuffle=on ./...
     ```
3. **Goroutine Leak Check**:
   - Use `go.uber.org/goleak` to verify that background goroutines spawned in tests are properly closed before exiting.
