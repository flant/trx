VERDICT: NEEDS_CHANGES

Not run (reviewer is read-only): `go test`, `GOOS=windows go build`. LSP unavailable (go 1.25 on PATH). Windows judged by reading: `process_unix.go` has `//go:build !windows`, `process_windows.go` selected by suffix, `syscall.Kill`/`Setpgid` only in the unix file — should build.

## Blocker
None.

## Should-fix

1. `cmd/trx/run.go:33-37` + `internal/command/command.go:66-70` — signal goroutine reads one signal, then `signal.Notify` keeps the default SIGINT disposition disabled forever. Hooks now run on `WithoutCancel` for up to 5 min (+10 s `WaitDelay`), so a second Ctrl-C does nothing. Before this batch, cancel = fast exit. `signal.Stop(signalChan)` after the first signal (restores default → second Ctrl-C kills) is the one-line fix.
2. `cmd/trx/run.go:146-147` — a run cancelled by SIGTERM (systemd stop, reboot) fails `Exec` with "context canceled" and calls `storeFailedTag`; the next scheduled run refuses the tag until `--force`. An interrupted deploy is not a failed tag. Guard with `ctx.Err() == nil`. (Failed-tag logic is from the previous batch; the interaction is new.)
3. `internal/git/client.go:129-134` — pre-releases skipped with no opt-in. A staging trx pointed at `-rc` tags now either deploys an older GA or fails "no semantic version tags found" on every run, silently after upgrade. README updated, but a config key (e.g. `repo.allowPrerelease`) is cheap.
4. `internal/storage/local/local_test.go:73` — `TestStoreSucceedTag_isAtomic` passes with `os.WriteFile` restored (content, one entry, 0644 all hold). Tests nothing the commit changed; at least assert no `last_processed_commit.*` temp leaks after a failed `write("")`, or drop the "atomic" claim.

## Nits

- `internal/git/client.go:163` — 15-min constant, no knob. A large repo on a slow link re-clones from scratch every run (temp dir removed) and never converges.
- `internal/git/client.go:207-209` — any `os.Stat` error (EACCES) is treated as "no clone", then `Rename` fails with an unrelated "unable to move the clone" message.
- `internal/config/config.go:20` — dropping `initial_last_published_git_commit` with `ErrorUnused: true` turns an undocumented but previously accepted key into a load error. Never in README; low risk.
- `internal/quorum/quorum.go:34` — `Error()` prefixes "quorum `x` error:" onto an inner error that already says "quorum `x` error reading GPG keys" → doubled text now that `run.go:123` returns `qErr` unwrapped.
- `cmd/trx/run.go:210-217` — repo `env` is loaded only when repo `commands` are used; README:81 implies repo env always applies. Pre-existing.
- `internal/command/process_unix.go:27` — `ESRCH` from `Kill` (group already gone) is reported by `Wait` if sh exited 0; map to `os.ErrProcessDone`.
- No test for `cloneRepo` (temp dir → rename) or the clone/fetch timeout; `PlainCloneContext` accepts a local path URL, so it is testable.

## Checked, fine

- Env precedence: README:81 and :128 both said operator wins; old code did the opposite. Flip matches docs.
- `worktree.Clean` (go-git 5.19.2 `worktree_status.go:95` passes `excludeIgnoredChanges=true`): `.gitignore`d files survive, like `git clean -fd` without `-x`. Test `client_test.go:107` fails on revert.
- run.go order: lock → storage → git(ctx) → target tag → last tag → executor → newer? → failed? → quorum → checkout → cmds → started → exec → store → success. No double `storeFailedTag`, hook names now correct, dead `IsNewerVersion` branch truly unreachable (`LessThanEqual` covers `Equal`).
- Revert-sensitive tests: template escape, upper-case env, mergeEnvs, annotated tag, prerelease, clean, pgroup kill (marker appears after 2 s with sh-only kill), hook-after-cancel, unnamed quorum, ssh key message, README load.
