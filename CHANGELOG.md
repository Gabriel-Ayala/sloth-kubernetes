# Changelog

## [0.5.1](https://github.com/chalkan3/sloth-kubernetes/compare/v0.5.0...v0.5.1) (2026-01-05)


### Bug Fixes

* resolve gofmt formatting issue in cluster_orchestrator.go ([fc5c9e5](https://github.com/chalkan3/sloth-kubernetes/commit/fc5c9e5054aacb50d0bf23766977acd00d4c339c))

## [0.5.0](https://github.com/chalkan3/sloth-kubernetes/compare/v0.4.1...v0.5.0) (2026-01-05)


### Features

* add Lisp evaluator with 70+ built-in functions and config validator ([e3a1281](https://github.com/chalkan3/sloth-kubernetes/commit/e3a1281f5dcfa34d94d38705427a39d3eeaee028))
* add Lisp evaluator with 70+ built-in functions and config validator ([da450fc](https://github.com/chalkan3/sloth-kubernetes/commit/da450fc6f53b975748f4243dfb87b3e6b25b782f))
* add state management, versioning, manifest registry, and audit logging ([c761a37](https://github.com/chalkan3/sloth-kubernetes/commit/c761a37418466268ce6f7d82996fd06e2a2d48ad))
* add state management, versioning, manifest registry, and audit logging ([6d26c33](https://github.com/chalkan3/sloth-kubernetes/commit/6d26c3388f77db2c8d4c5c6c99c2b582eb181626))

## [0.4.1](https://github.com/chalkan3/sloth-kubernetes/compare/v0.4.0...v0.4.1) (2026-01-04)


### Bug Fixes

* resolve CI pipeline failures and test issues ([ab78da1](https://github.com/chalkan3/sloth-kubernetes/commit/ab78da1346a728fc2317a31e34a9ac5f6e9c6635))
* resolve CI pipeline failures and test issues ([92736ec](https://github.com/chalkan3/sloth-kubernetes/commit/92736eca9b508cb3ec952b090293888516f6d939))

## [0.4.0](https://github.com/chalkan3/sloth-kubernetes/compare/v0.3.0...v0.4.0) (2026-01-04)


### Features

* Add enhanced one-line installation script ([#4](https://github.com/chalkan3/sloth-kubernetes/issues/4)) ([7f1ef9f](https://github.com/chalkan3/sloth-kubernetes/commit/7f1ef9fdd2908482900adfcc13c708a6d4b140f7))
* add goreleaser, Dockerfile, and comprehensive test suite ([c4f32ee](https://github.com/chalkan3/sloth-kubernetes/commit/c4f32eec7676988249d0c8a1e18187b561084ec0))
* Add helm and kustomize wrapper commands ([#9](https://github.com/chalkan3/sloth-kubernetes/issues/9)) ([a01afac](https://github.com/chalkan3/sloth-kubernetes/commit/a01afac19c809e5c9f60726960bfa3f9a68e9abd))
* Add login command for secure credential management ([#2](https://github.com/chalkan3/sloth-kubernetes/issues/2)) ([4882869](https://github.com/chalkan3/sloth-kubernetes/commit/4882869bea04cdf512d396160cf2dd015c7deae7))
* Improve Salt API automation and cluster lifecycle management ([#7](https://github.com/chalkan3/sloth-kubernetes/issues/7)) ([66dac2b](https://github.com/chalkan3/sloth-kubernetes/commit/66dac2bbb85e32079eed19cf039ed63f48e7cc14))
* **salt:** switch to sharedsecret auth for reliable API access ([da1750c](https://github.com/chalkan3/sloth-kubernetes/commit/da1750c696e1277cf74e99f0df968a5363342114))


### Bug Fixes

* **ci:** Disable Go cache to fix tar extraction errors in release workflow ([#12](https://github.com/chalkan3/sloth-kubernetes/issues/12)) ([3496a4d](https://github.com/chalkan3/sloth-kubernetes/commit/3496a4d155c810a7dc5f1686f18ead1ca6d384c0))
* **docs:** Enable automatic GitHub Pages setup in workflow ([#11](https://github.com/chalkan3/sloth-kubernetes/issues/11)) ([fd1529c](https://github.com/chalkan3/sloth-kubernetes/commit/fd1529c7441745c9a5b6c23d044aeab4a5975912))
* **e2e:** Fix minion-0 connection timing issue ([947e682](https://github.com/chalkan3/sloth-kubernetes/commit/947e682b44bbab7e127c54a860b49e6244b1b143))
* **e2e:** Fix Salt comprehensive E2E test to pass all 13 phases ([e847e29](https://github.com/chalkan3/sloth-kubernetes/commit/e847e29a8bf8799f41fa7c4a29d7bf03edd29d7f))
* **e2e:** Increase Salt E2E test timeouts and instance sizes ([7cd4442](https://github.com/chalkan3/sloth-kubernetes/commit/7cd44425eff765c50557c1c43aa091d951a7f4c2))
* resolve gofmt and go vet issues ([4138a37](https://github.com/chalkan3/sloth-kubernetes/commit/4138a37c04fb42420c7cf1f2217f8d64c1358dc4))


### Performance Improvements

* optimize provisioning timeouts and remove hard sleeps ([101b4b0](https://github.com/chalkan3/sloth-kubernetes/commit/101b4b0d7007952f8e82e0163fb9be9379bcab34))
