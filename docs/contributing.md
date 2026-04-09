# Contributing

## Code Style and Conventions

This project follows standard Go conventions as outlined in [Effective Go](https://go.dev/doc/effective_go) and the [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments).

### Naming

- Use **MixedCaps** or **mixedCaps** rather than underscores for multi-word names
- Acronyms should be all capitals (e.g., `URL`, `HTTP`, `API`)
- Interfaces with a single method should be named with the method name plus the `-er` suffix (e.g., `Reader`, `Writer`)
- Package names should be short, concise, lowercase, and without underscores or mixedCaps

### Code Organization

- Organize imports into groups: standard library, third-party, local packages
- Use `make verify-fmt` to format all code before committing
- Run `make vet` to catch common mistakes

### Error Handling

- Always check errors; don't use `_` to discard errors unless you have a good reason
- Provide context when returning errors using `fmt.Errorf` with `%w` for wrapping
- Use meaningful error messages that help debugging
