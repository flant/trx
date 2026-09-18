VERDICT: APPROVE

Tests: `go test -count=1 -race ./internal/...` all pass. None vacuous: the long-line test would hang ≥30s and fail under the old scanner (200 KB > pipe buffer); the no-checkout test asserts HEAD unchanged and `untagged.txt` present, which the old `GetTargetGitObject` would have removed; config duplicate tests hit real fingerprint comparison (fresh key per call).

## Priority 1 — regressions

None blocking.
- Repo `trx.yaml` still works: `cmd/trx/run.go:125` checkout precedes `getCmdsToRun` at `:130`, which passes `gitClient.RepoPath` — same value old `command.WorkDir` held (`openGitRepo` set it). `NewRunnerConfig(repoPath, …)` unchanged in semantics.
- onCommandStarted/Success/Failure cwd = clone (`run.go:128 executor.WorkDir = gitClient.RepoPath`), env merge unchanged. onCommandSkipped/onQuorumFailure cwd changed to operator cwd — intentional, README documents it.
- Exit codes: all errors → `log.Fatal` → 1, unchanged. Failed-tag skip exits 1 without hooks; documented.
- `--force` bypasses both the not-newer check and the failed-tag check; storeFailedTag on success path cleared by `StoreSucceedTag` (`local.go:75`).

## Should-fix

1. `internal/config/config.go:208-211` — an entry containing two entities is counted as 2 distinct keys, but the verifier (`trdl pkg/pgp/util.go:43`) drops the whole *entry* after one match, so `minNumberOfKeys: 2` with one two-key entry passes validation yet can never verify. Fail-closed, but confusing. Reject multi-entity entries or count 1 per entry.
2. `internal/command/command.go:113` — `cmd.Run` blocks until all pipe writers close; SIGTERM kills `sh` but a backgrounded grandchild keeps the pipe open → hang holding the lock (the very failure #17 targets). Pre-existing on stderr, now also stdout. Set `cmd.WaitDelay`.
3. `cmd/trx/run.go:103` — failed tag keyed by name only. With Force+Prune a re-pointed tag of the same name is fetched but refused until `--force`. Store `tag@commit` if retagging is a supported recovery.

## Nits

- `internal/storage/local/local.go:41` — migration silent; `migrateLegacyClone` logs, this doesn't. If the legacy dir was shared by two same-name repos the copied `last_processed_commit` may be the other repo's (redeploy if lower, stuck if higher). Unfixable automatically; log it so operators can spot it.
- `internal/command/command_test.go:20,37` — `log.SetOutput(nil)` panics on any later log write; use `io.Discard`.
- `internal/command/command.go:112` — stderr now streams live *and* is repeated (8 KiB tail) on failure → duplicate lines in logs.
- `internal/.DS_Store` committed in 6453ddc; add to `.gitignore`.
- `internal/command/command.go:31` — `os.Getwd` failure now fatal (deleted cwd); old code never needed cwd.

## Open questions

- Migration runs before `locker.Acquire` (`run.go:43` vs `:51`); two concurrent first runs both write identical state — benign, but ordering could be swapped.
