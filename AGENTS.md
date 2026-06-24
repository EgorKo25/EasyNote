                                          # Test Writing Rules

## General
- Do not change production code when the task is to add or update tests.
- Do not create thin wrapper/helper functions that only forward arguments to another function, add no domain decision, and exist only to split lines or rename a call. Inline that logic at the call site or keep one meaningful function with real responsibility.
- Strictly forbidden: extracting a single-use function that hides a linear fragment of its only caller and replaces the caller's local values with a long parameter list. A function must not exist merely to move SQL, object construction, formatting, validation, or a sequence of calls out of one call site. Inline such logic. Keep a single-use function only when it owns a clear independent domain or technical responsibility, establishes a real dependency boundary, or is required as a callback. Parameter grouping does not justify the extraction by itself.
- Tests must be honest. If a test fails because of a defect in production code, leave the failing test as is and report the defect.
- Always describe what you are going to do before starting test work.
- Always include a matrix of completed work in the final result for test-related tasks.
- After each completed task, run tests and run the linter for the whole repository with the repository configuration.
- Always run `golangci-lint run` for the whole repository. Do not limit lint checks to individual packages or directories.
- If tests or lint fail, do not change production code and do not adapt tests to hide the problem. Describe what failed and why, then wait for the user's decision.
- Ask for the user's permission before changing any system applications, utilities, or files outside the repository.
- Do not change the linter configuration.
- Do not add `nolint` directives to production files. `nolint` is allowed only in test files.
- After completing and pushing any task that belongs to a pull request, check the pull request CI status and report whether every required check passed.
- Final report matrices must always be written in Russian.
- Do not extract a helper that only wraps one or two function calls or only moves a small linear fragment out of the caller when it is used fewer than three times. Keep the logic inline unless the helper owns a real domain responsibility, establishes a real dependency boundary, or is required as a callback.

## Token Efficiency And No Background Reasoning

* Do not output background reasoning, internal chain of thought, detailed reflections, or “thinking out loud” unless the user explicitly asks for it.
* Do not waste tokens on obvious explanations, task restatements, long introductions, or intermediate reasoning that does not help complete the task.
* Before starting work, briefly describe the action plan, but do not reveal detailed internal reasoning.
* During the work, report only meaningful statuses: what has been completed, what is blocked, which checks were run, and what the result was.
* In the final response, include only useful results: changed files, completed checks, errors, coverage, branch name, pull request, and the completed work matrix.
* Do not write long explanations for choices when the solution is standard for the repository or already follows from the task rules.
* If a defect or failed test needs to be explained, keep it brief and specific: what failed, where it failed, and why it is related to production code or the test.
* Do not imitate “thinking”, “analysis”, or “hypothesis checking” in text. Instead, perform the actions and briefly report the result.


## Task Execution

* Every task must be completed fully. Do not stop after a partial implementation, a partial test set, or a partial refactor unless the task is blocked by a real external issue.
* Do not mark the task as completed if any required step was skipped, including tests, lint, coverage check, branch creation, or pull request creation.
* If the task cannot be completed 100%, clearly report what is incomplete, why it is incomplete, what was already done, and what exact decision is required from the user.
* Do not silently downgrade the task scope. If the requested scope is too large, still make the best complete attempt and report remaining gaps explicitly.
* Do not replace the requested implementation with an easier adjacent solution unless it preserves the same behavior and satisfies the original task.
* Do not leave TODO comments, temporary code, debug prints, unused files, or commented-out code after completing the task.

## Branch And Pull Request Workflow

* Every task must be performed in a separate Git branch.
* The branch name must be short, meaningful, and related to the task.
* Do not work directly in `main`, `master`, `develop`, or another shared integration branch.
* After the task is completed, open a pull request for the branch.
* The pull request must contain a clear summary of the changes, the completed test/lint/coverage checks, and any known issues.
* When pull request review comments are present, read every comment, fix every valid remark, and reply under each comment with a brief summary in the same language as the original comment.
* If branch creation or pull request creation is impossible because of missing permissions, missing remote configuration, or unavailable tools, report this explicitly and do not claim that the task is fully completed.

## Scope Control

* Do not change unrelated files.
* Do not perform opportunistic refactoring outside the requested task.
* Do not rename public APIs, packages, files, database fields, configuration keys, or exported types unless the task explicitly requires it.
* Do not introduce new dependencies unless they are necessary for the task and the reason is explained.
* Do not change generated files manually. Regenerate them through the repository's documented generation commands.
* Do not change migrations, schemas, or public contracts when the task is only about tests.

## Final Report

* The final response must include:

    * what was changed;
    * what files were changed;
    * what tests were added or updated;
    * what commands were run;
    * whether tests passed or failed;
    * whether `golangci-lint run` passed or failed;
    * current target package coverage;
    * a matrix of completed work;
    * the branch name;
    * the pull request link or an explicit reason why the pull request was not created.

## Test Reliability

* Tests must be deterministic and must not depend on wall-clock time, network access, external services, shared local state, or execution order unless the task explicitly requires integration testing.
* Use clock abstractions or controlled test time where time-dependent behavior is tested.
* Clean up temporary files, database records, goroutines, channels, and test resources.
* Do not use sleeps as synchronization in unit tests unless there is no reasonable alternative and the reason is explained.
* Do not make tests pass by weakening assertions.
* Do not delete or skip existing failing tests unless the user explicitly approves it.

## Git Hygiene

* Before starting, check the current Git status.
* Do not overwrite or remove user changes.
* If there are unrelated uncommitted changes, leave them untouched and report them.
* Keep commits focused on the requested task.
* Do not include local configuration, secrets, logs, binaries, coverage files, or temporary artifacts in the commit.


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
