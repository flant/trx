# Changelog

## [2.0.0](https://www.github.com/flant/trx/compare/v1.0.0...v2.0.0) (2026-09-19)


### ⚠ BREAKING CHANGES

* **quorum:** The existing cases all run with NumberOfKeys: 1, so they cover a mixed key list rather than a quorum: the second signature of a quorum lives in the git-signatures note, not in the tag object. Add a 2-of-2 case with an EdDSA tag signature and an RSA note signature, plus the negative case where only one of the two signing keys is trusted.

### Bug Fixes

* **command:** fail on an unknown template variable ([f6b265e](https://www.github.com/flant/trx/commit/f6b265ebe89a9a60c2991d2b8ccd544fca64a86d))
* **command:** render commands with text/template ([4f4543b](https://www.github.com/flant/trx/commit/4f4543b034237027da7c17252cc6d2c6ee45d203))
* **command:** signal the whole process group, keep hooks alive ([11e8e8e](https://www.github.com/flant/trx/commit/11e8e8e728ff4d722d6f47d1679fe3777fec70ed))
* **command:** upper-case env names in one place, let the operator win ([6c32151](https://www.github.com/flant/trx/commit/6c32151d330792d6e8e9a6b345a1616e3134e4a5))
* **config:** keep accepting initial_last_published_git_commit ([5b8f75a](https://www.github.com/flant/trx/commit/5b8f75a5a2a93e09383e0885ec1120b346d6a5c3))
* **git:** bound clone and fetch, and repair a broken clone ([7e9f572](https://www.github.com/flant/trx/commit/7e9f572122910dd393e7cb0d15dca1843fb8093e))
* **git:** clean the worktree when checking out the target tag ([6101d8c](https://www.github.com/flant/trx/commit/6101d8c6111ac10d478b65c9a9535cfd726faaaa))
* **git:** deploy the newest tag again, pre-release or not ([469e8c2](https://www.github.com/flant/trx/commit/469e8c2186b9141364768775ce2604704a927fc7))
* **git:** do not deploy a pre-release as a release ([6969ea1](https://www.github.com/flant/trx/commit/6969ea1732ed8fccc4583bc489f93870d4f60415))
* **git:** resolve an annotated tag to its commit ([3ec414f](https://www.github.com/flant/trx/commit/3ec414f4b67afb990eeb79b6dec751483579d050))
* **lock:** skip locking with --disable-lock and fail on a lost race ([#34](https://www.github.com/flant/trx/issues/34)) ([418047c](https://www.github.com/flant/trx/commit/418047c0c0c7957536c58e650747d28d86758143))
* print hook errors, name the right hooks, drop dead code ([4256e98](https://www.github.com/flant/trx/commit/4256e984018a159914fc1f409111dd9b4f2e3b05))
* **quorum:** do not panic on a quorum without a name ([1dc7384](https://www.github.com/flant/trx/commit/1dc738423066449e20b8b7ceb24b48e1759d2ab5))
* **quorum:** support EdDSA (Ed25519) GPG keys in signature verification ([b32caf4](https://www.github.com/flant/trx/commit/b32caf4a053170c505c8e39f6e4644d0599a1afb))
* **quorum:** support EdDSA (Ed25519) GPG keys in signature verification ([d8ee7a4](https://www.github.com/flant/trx/commit/d8ee7a4f25d036053d1522570034890ed30a332d))
* **quorum:** support EdDSA (Ed25519) GPG keys in signature verification ([#12](https://www.github.com/flant/trx/issues/12)) ([b32caf4](https://www.github.com/flant/trx/commit/b32caf4a053170c505c8e39f6e4644d0599a1afb))
* **quorum:** warn about a duplicate GPG key instead of refusing to start ([f454712](https://www.github.com/flant/trx/commit/f45471206a22ba6963847e28ac32ea8c214c97f4))
* report the locker and ssh key errors instead of ignoring them ([6caa0b8](https://www.github.com/flant/trx/commit/6caa0b86f514e8fbe54bba20b70d51b51c8b7c57))
* **run:** retry a tag that failed again ([e23b5a9](https://www.github.com/flant/trx/commit/e23b5a9965cdeaeba86be75a363032e264cb8056))
* **storage:** write the state atomically ([b39a881](https://www.github.com/flant/trx/commit/b39a881d2092e5791b7cffe2ba3d103f2ec65db1))
* the blocking findings of the main audit ([#37](https://www.github.com/flant/trx/issues/37)) ([c8c3cd9](https://www.github.com/flant/trx/commit/c8c3cd974b473dd0cac0b28a9a30fbc3dfb10f44))


### Tests

* **quorum:** cover a real two-signature quorum, generate the test key ([ad9f231](https://www.github.com/flant/trx/commit/ad9f23118e37c57458c3c6f3d5eee0050eeb2e60))

## 1.0.0 (2025-03-21)


### Features

* add commands to main config ([a683943](https://www.github.com/flant/trx/commit/a683943520ea4f46c0981613b1901edfa0ac319a))
* add execution lock ([826065b](https://www.github.com/flant/trx/commit/826065b7eb5aa57c8a0c9a0cca6130de38bb3cc1))
* add force flag ([537b739](https://www.github.com/flant/trx/commit/537b73980d69739c98043605afde90e873652c8f))
* add force flag ([0ade769](https://www.github.com/flant/trx/commit/0ade7697cf1b5797d0fdd13f935a3cdc0a26ab09))
* add graceful shutdown ([5b12819](https://www.github.com/flant/trx/commit/5b12819d6754d6a5703a1b7a330b8469fb943010))
* add graceful shutdown ([e32ddaf](https://www.github.com/flant/trx/commit/e32ddafaf3363450e2d261002babf77616ccaf67))
* add possibility to execute command from cli ([554770e](https://www.github.com/flant/trx/commit/554770e80e00e9a8e3fa325445bda8240499102c))
* add start hook, start time, env expander ([9d7775f](https://www.github.com/flant/trx/commit/9d7775f2e8ef4b7852c74023b122369c791daa5f))
* add stderr logging ([d0d9f57](https://www.github.com/flant/trx/commit/d0d9f57b614faec6cb97d618cffcbb013090159e))
* log to stdout ([4c046ee](https://www.github.com/flant/trx/commit/4c046ee0cdad787f7283d4801c87cad992a538d2))
* log to stdout ([d5f4152](https://www.github.com/flant/trx/commit/d5f4152358eb6b94a1c7c7c0761c6071929a9c1d))


### Bug Fixes

* fix hook panic ([6f7cc78](https://www.github.com/flant/trx/commit/6f7cc78c437a09db14b37895939a667c94d879da))
