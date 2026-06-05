                                          # Test Writing Rules

## General
- Do not change production code when the task is to add or update tests.
- Tests must be honest. If a test fails because of a defect in production code, leave the failing test as is and report the defect.
- Always describe what you are going to do before starting test work.
- Always include a matrix of completed work in the final result for test-related tasks.
- After each completed task, run tests and run the linter for the whole repository with the repository configuration.
- Always run `golangci-lint run` for the whole repository. Do not limit lint checks to individual packages or directories.
- If tests or lint fail, do not change production code and do not adapt tests to hide the problem. Describe what failed and why, then wait for the user's decision.
- Ask for the user's permission before changing any system applications, utilities, or files outside the repository.
- Do not change the linter configuration.

## Test Structure
- Use the table-driven approach in every test.
- Each test must contain at least 4 positive cases and 2 negative cases.
- Test case names must be meaningful and exhaustive.
- Do not move test cases out of the test function.
- Do not use `testCase := testCase`; rely on the current Go language behavior.
- Avoid `switch` dispatch over string operation kinds inside tests. Prefer separate table-driven tests by operation or direct scripts/functions in the test case.

## Assertions
- Do not use `wantErr` or similar boolean flags for error checks.
- If error verification is needed, define an `expected` structure with a single `expectedErrText string` field and compare errors with `require.EqualErrorf`.
- In new table-driven tests, do not put two or more error fields into `expected`. Prefer a single `expectedErrText string` field; use an empty string when no error is expected.
- Prefer `require.EqualErrorf` or `require.NoError` over branching on sentinel error fields in table assertions.
- Prefer assertion helpers from `require` when they simplify the test without weakening it.
- Do not use fallback branches like checking `nil` versus empty slices when an explicit expected slice can be provided. Put the expected slice directly into the test case.

## Fixtures And Helpers
- Do not create more than one helper function.
- If a fixture is required, place it in `fixture_test.go` next to the test file unless explicitly asked to keep it in the same test file.
- If setup logic is needed, prefer adding it to the test case as a function such as `beforeScript func(t *testing.T, ...)` and call it inside the test body.
- Every package dependency interface must have a nearby `go:generate` directive for mock generation.
- Interfaces that only document a local implementation detail and do not split package dependencies or participate in tests may omit `go:generate`.
- Do not replace functional dependencies with handwritten stubs in tests. Close the dependency with an interface, generate a mock for it, and use that mock in tests.
- Every generated mock expectation must end with `Times(...)` with an explicit expected call count.
- When a test expects more than one mock call in a specific order, wrap those expectations in `gomock.InOrder`.

## Coverage
- Coverage for the target package must not be below 75%.

## Scheduler Architecture
- Scheduler processing must stay inside the scheduler package logic unless there is a real implemented dependency boundary.
- In Go files, put every interface declared in the file near the top of that file, before constructors, structs, and function bodies.
- Scheduler runtime must be split by responsibility: runner/tick-loop, ready-reminder sending, retry/backoff policy, clock abstraction, and dependency interfaces.
- Scheduler must not reintroduce the old heap/cache queue model or public `Next(ctx)` due-reminder API.
- Retry/backoff must be hidden behind an internal project interface; external backoff library APIs must not leak into runner orchestration.
- Do not add the global logger to structs. Get named loggers at the call site where logging is performed.
