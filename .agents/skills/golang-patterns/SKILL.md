---
name: golang-patterns
description: "Idiomatic Golang design patterns: functional options, API constructors, struct and interface guidelines, error handling flow, resource lifecycle, performance optimizations, and testability. Apply when refactoring, writing new APIs, designing packages, or deciding on structural patterns."
user-invocable: true
license: MIT
compatibility: Designed for AI coding agents and Go developers.
---

# Go Design Patterns & Idioms

This workspace skill documents structural, behavioral, and idiomatic patterns for Go applications, aiming to enforce codebase cleanliness, performance, and predictability.

## 1. API & Constructor Patterns

### Functional Options (Preferred)
Use **Functional Options** for constructors when configuration parameters can grow or change. This prevents breaking API changes in the future.

```go
type Client struct {
    endpoint string
    timeout  time.Duration
    maxConns int
}

type Option func(*Client)

func WithTimeout(t time.Duration) Option {
    return func(c *Client) {
        c.timeout = t
    }
}

func WithMaxConns(n int) Option {
    return func(c *Client) {
        c.maxConns = n
    }
}

func NewClient(endpoint string, opts ...Option) *Client {
    c := &Client{
        endpoint: endpoint,
        timeout:  30 * time.Second, // Defaults
        maxConns: 10,
    }
    for _, opt := range opts {
        opt(c)
    }
    return c
}
```

### Constructor Design
1. **Validation**: If validation is required, the constructor MUST return `(*T, error)` rather than failing at runtime.
2. **Explicitness**: Avoid using global structures or singletons. Inject dependencies through the constructor.
3. **Avoid `init()`**: Avoid using `init()` functions for setup. They hide package-level side-effects, cannot return errors, and make tests unpredictable. Use explicit, user-invoked initialization functions.

---

## 2. Struct and Interface Design

1. **Accept Interfaces, Return Concrete Types**:
   - Functions should accept interface parameters (maximizing reuse) and return concrete structs (making downstream access simpler).
2. **Keep Interfaces Small**:
   - Prefer single-method interfaces (e.g., `io.Reader`, `io.Writer`) or interfaces with 2-3 methods. Large interfaces are hard to implement and mock.
3. **Compile-Time Interface Verification**:
   - Ensure concrete types implement target interfaces at compile-time:
     ```go
     var _ MyInterface = (*MyConcreteType)(nil)
     ```
4. **Enums**:
   - Define custom type aliases for enums.
   - Always start enum blocks at 1 (or define an `Unknown` sentinel at 0). Go's zero-value initialization otherwise causes variables to default to the first enum value.
     ```go
     type Status int
     const (
         StatusUnknown Status = iota
         StatusPending
         StatusActive
     )
     ```

---

## 3. Error Handling and Control Flow

1. **Early Returns (Line of Sight)**:
   - Handle error conditions immediately and return. Keep the happy path flat (un-indented).
2. **Contextual Wrap**:
   - Wrap errors with additional context when passing them up the stack, using `%w`:
     ```go
     if err != nil {
         return fmt.Errorf("fetching resource: %w", err)
     }
     ```
3. **Panic is NOT for Error Flow**:
   - Only panic for unrecoverable programmer errors (e.g., array index out of bounds, nil pointer dereference, setup bugs). Never use panics for expected edge cases.

---

## 4. Lifecycles and Resource Management

1. **Deallocate / Close Immediately**:
   - Use `defer` to clean up resources (connections, files, mutexes) immediately after successful initialization.
     ```go
     resp, err := http.Get(url)
     if err != nil {
         return err
     }
     defer resp.Body.Close()
     ```
2. **Timeouts Everywhere**:
   - Every external network request, channel select, and context block MUST have a timeout or context deadline. Unbounded resources will eventually block the whole system.
3. **Graceful Shutdown**:
   - Pass a `context.Context` to service runners, listening to `ctx.Done()` for graceful termination.

---

## 5. Performance and Allocation Rules

1. **Strings Concatenation**:
   - Use `strings.Builder` instead of `+` when concatenating strings in loops to minimize allocations.
2. **Pre-allocate slices**:
   - When the size of a slice is known beforehand, initialize it using `make([]T, 0, capacity)` to avoid reallocation overhead.
3. **Regex Compilation**:
   - Never compile regexes inside functions that run repeatedly. Compile them once using `regexp.MustCompile` at package-level.
