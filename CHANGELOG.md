# Changelog

## [1.0.0](https://github.com/bluegardenproject/stac-man/compare/v0.2.1...v1.0.0) (2026-05-04)


### ⚠ BREAKING CHANGES

* **tui:** bare `sm` on a TTY now launches the interactive cockpit instead of printing help. Run `sm --help` (or any explicit subcommand) for the previous behaviour. Non-TTY invocations are unchanged.

### Features

* **config:** load user config and add `sm config` command ([f5d1153](https://github.com/bluegardenproject/stac-man/commit/f5d1153372bbd30e53a0c2b294f74ad4aedcc63a))
* **docs:** add stac-man branding to site and README ([3080acd](https://github.com/bluegardenproject/stac-man/commit/3080acd9ad62f14b97a097c6da70208486bb2b99))
* **docs:** align site theme with stac-man logo palette ([0566a54](https://github.com/bluegardenproject/stac-man/commit/0566a5415f33ddf37ca9a619fc9d3aa17c07296a))
* **install:** broaden shell coverage and add uninstaller ([61b6d57](https://github.com/bluegardenproject/stac-man/commit/61b6d5708de3cef9ff40a951055cb016471a3a40))
* **tui:** add interactive cockpit ([2542854](https://github.com/bluegardenproject/stac-man/commit/254285457cc2889a17295725efbde248ee764c7a))
* **update:** background notifier for new releases ([f77edf8](https://github.com/bluegardenproject/stac-man/commit/f77edf8f84ed44760da45daaab06d08a0ef6d91a))


### Documentation

* ship cockpit, rename docs/site to docs/web, enforce pasteable shell blocks ([1047280](https://github.com/bluegardenproject/stac-man/commit/10472805120a9fc45b2346fa3d8708f6729bd9a4))


### Miscellaneous

* **hooks:** add pre-commit gofmt check ([dc865ac](https://github.com/bluegardenproject/stac-man/commit/dc865ace6174879896d325bf19c1146b47557fc5))

## [0.2.1](https://github.com/bluegardenproject/stac-man/compare/v0.2.0...v0.2.1) (2026-05-02)


### Bug Fixes

* point repo URLs at bluegardenproject/stac-man ([36ad8d6](https://github.com/bluegardenproject/stac-man/commit/36ad8d6483e4ec12b9759ba865989e15748958ca))
* rename Go module to github.com/bluegardenproject/stac-man ([dbd2c1a](https://github.com/bluegardenproject/stac-man/commit/dbd2c1ab7286caf273045e6c2c72fe72bffe2de2))


### Miscellaneous

* **main:** release 0.1.0 ([e47fec9](https://github.com/bluegardenproject/stac-man/commit/e47fec9c35c383fd28ebbb4b4ebe8cc040f82e57))
* **main:** release 0.2.0 ([3d9e736](https://github.com/bluegardenproject/stac-man/commit/3d9e736f0635b08f16602cdef7e813e31fb5585b))

## [0.2.0](https://github.com/bluegardenproject/stac-man/compare/v0.1.0...v0.2.0) (2026-05-02)


### Features

* inject auto-managed stack table into PR bodies ([784e0e7](https://github.com/bluegardenproject/stac-man/commit/784e0e76d58822d2aae427f1b8ea2dfb526f0b9d))
* **log:** add --json and --porcelain output formats ([ccf7262](https://github.com/bluegardenproject/stac-man/commit/ccf7262a3a260084f25c949020af7dbf7a2371fd))
* **progress:** show spinner during sm sync ([60509b5](https://github.com/bluegardenproject/stac-man/commit/60509b552a9a15152608ee582f06f295ddcfa153))
* **progress:** show spinner while pushing and fetching gh data ([5333494](https://github.com/bluegardenproject/stac-man/commit/53334948c4734d5d2b0f02add2db3ffdf9ebb96d))
* show CI + mergeability in sm log/doctor and add submit --no-restack ([ae55d16](https://github.com/bluegardenproject/stac-man/commit/ae55d163fc9bee5804419b525744b552407f3b9f))


### Bug Fixes

* **sync:** stop resurrecting deleted children during merged cleanup ([ab6d1ef](https://github.com/bluegardenproject/stac-man/commit/ab6d1ef3e747f67ce55ddaea6637467316ec2dab))


### Code Refactoring

* **log:** extract logData / renderLogTree seam from Log ([6058c01](https://github.com/bluegardenproject/stac-man/commit/6058c01e4ee10dc4434054ed880a14b672ade19e))
* switch CI/mergeability glyphs to coloured badges ([b266c78](https://github.com/bluegardenproject/stac-man/commit/b266c78144b969270a62364fe2cd76613fc5ee35))
* switch open PR pill from green to pink ([e44ac14](https://github.com/bluegardenproject/stac-man/commit/e44ac14db7fdda26cbac86c6713e526d8b240bc4))
* **sync:** drop static "pulled trunk" line from summary ([0a3493a](https://github.com/bluegardenproject/stac-man/commit/0a3493ab34c606fddc184dc36c7e1519c21080c6))


### Documentation

* document CI/mergeability badges, --no-restack, and PR body stack table ([9ac9da1](https://github.com/bluegardenproject/stac-man/commit/9ac9da10b98b490e349aa76314bcb1e19207c033))
* refresh sm log snippets in recipes and first-stack tutorial ([94dfbf6](https://github.com/bluegardenproject/stac-man/commit/94dfbf6c5f6fddeb73ad9401bceec160e3208990))


### Miscellaneous

* **cursor:** add caveman mode rule and skill ([3796624](https://github.com/bluegardenproject/stac-man/commit/3796624a77ec9c88167aaa95f868a82fec5b145f))

## [0.1.0](https://github.com/bluegardenproject/stac-man/compare/v0.0.1...v0.1.0) (2026-04-28)


### Features

* **checkout:** bubbletea picker for sm checkout with no args ([be3d33b](https://github.com/bluegardenproject/stac-man/commit/be3d33b510aa6c48eefd87808955b96bab6faa0c))
* **checkout:** bubbletea picker for sm checkout with no args ([b579181](https://github.com/bluegardenproject/stac-man/commit/b5791814147d92054157deb7f1491467d6bd0cd5))
* **doctor:** detect parent SHA drift after history rewrites ([#3](https://github.com/bluegardenproject/stac-man/issues/3)) ([0977d9e](https://github.com/bluegardenproject/stac-man/commit/0977d9eaa21e90a7baa3322a9fc27b94920e971e))
* **gh:** add gh CLI wrapper for repo info and PR ops ([0398742](https://github.com/bluegardenproject/stac-man/commit/0398742c39dfea2f3b57899658a7fd73f7f7da8a))
* **git:** add git wrapper with typed accessors and fake-runner tests ([1e1c9b5](https://github.com/bluegardenproject/stac-man/commit/1e1c9b5bc4b828af0981bd2d3d9492dc4271dd94))
* **release:** set up versioning, install scripts, and release-please ([37b42f0](https://github.com/bluegardenproject/stac-man/commit/37b42f0b7128acfe1cee87bd40dfce7e5aff9177))
* **restack:** add restack engine with conflict pause/resume and modify command ([c83f79c](https://github.com/bluegardenproject/stac-man/commit/c83f79c45eb74769fd860d8a47974953b20f426f))
* **service:** add doctor command and global yaml config loader ([9447039](https://github.com/bluegardenproject/stac-man/commit/94470392e42b804a38701bcd25b68520a5b90ed6))
* **service:** add navigation commands (checkout, up/down/top/bottom) ([630b52e](https://github.com/bluegardenproject/stac-man/commit/630b52e7d2e3805e85d25071eac929789c9919bb))
* **service:** add parent, children, fold commands ([22fef98](https://github.com/bluegardenproject/stac-man/commit/22fef98c2a174a990d80c0863dacfda0d42b73ff))
* **service:** add service layer and create/track/untrack/log commands ([a080992](https://github.com/bluegardenproject/stac-man/commit/a08099206a918a46d93a71fe04cbee4313248104))
* **service:** add sync and submit commands ([0962810](https://github.com/bluegardenproject/stac-man/commit/0962810999f6af59a540479e4955744848a5fc76))
* **stack:** add store interface, gitconfig+memory impls, and graph domain ([0ad7b9d](https://github.com/bluegardenproject/stac-man/commit/0ad7b9d68c0401832216a0cd2b27e37d7e31cab5))
* **ui:** port synthwave theme from github-butler ([eff4069](https://github.com/bluegardenproject/stac-man/commit/eff406968acff0282500fca8d1d59cb8d5ed23ab))
* v2.0 graphite parity additions ([67a75c8](https://github.com/bluegardenproject/stac-man/commit/67a75c8656af40fedf20843b841c4c33d0ac5cf4))


### Bug Fixes

* **git:** set GIT_EDITOR=: so non-interactive git invocations don't break ([3cfbdf9](https://github.com/bluegardenproject/stac-man/commit/3cfbdf99190bca4bda0c54926e1e2441b3504a8d))
* **git:** set GIT_EDITOR=: so non-interactive git invocations don't break ([d84f985](https://github.com/bluegardenproject/stac-man/commit/d84f985ca32941988001c0030512564c144b853c))
* **service:** -a stages tracked files only, opt-in for untracked ([bf9718c](https://github.com/bluegardenproject/stac-man/commit/bf9718c780e2f0fbcfd241774591c29925ef29b3))
* **service:** refuse amend on empty branches and unblock -c ([8ec4224](https://github.com/bluegardenproject/stac-man/commit/8ec4224dedc5320d36660495f83476ba624cd228))
* **service:** sm move now triggers a real rebase even on ancestor base ([3d9f9e6](https://github.com/bluegardenproject/stac-man/commit/3d9f9e6e0f05552b1de6588e2b7dd04fe7e82e0a))
* **service:** sm move now triggers a real rebase even on ancestor base ([8a27e3a](https://github.com/bluegardenproject/stac-man/commit/8a27e3a023372ed8e158a25daf9b806e1100dbe9))
* **submit:** align --stack and plain submit with Graphite parity ([d20db74](https://github.com/bluegardenproject/stac-man/commit/d20db74a8137fd946601bf92d4ba1af9ce534d74))
* **submit:** derive PR title and body from commits, not branch name ([8a2980b](https://github.com/bluegardenproject/stac-man/commit/8a2980b3fe66c961361647bdb55b2539a20e1a3b))
* **submit:** include unsubmitted ancestors when --stack is set ([ae85b00](https://github.com/bluegardenproject/stac-man/commit/ae85b00c5add974786bd11f2a7764cdb4812d6fc))
* **sync:** detect squash & merge-commit landings via gh PR state ([3b46cbd](https://github.com/bluegardenproject/stac-man/commit/3b46cbda614f4add257fb41ecdd87ad2c3115d1b))
* **sync:** retarget child PR bases on GitHub when their parent merges ([95fe34a](https://github.com/bluegardenproject/stac-man/commit/95fe34aae999a63fbf078e2988b79e846a15f065))


### Documentation

* add user and dev skill files plus README rewrite ([21d641c](https://github.com/bluegardenproject/stac-man/commit/21d641ce1e5288516f885ca7ee32168f6132bcb6))
* drop graphite references and internal roadmap from README ([c6a25d8](https://github.com/bluegardenproject/stac-man/commit/c6a25d8f81c776d755af84e48287437b42cad8d8))
* **site:** scaffold VitePress site, drift skill, and shared palette ([f549310](https://github.com/bluegardenproject/stac-man/commit/f54931030fe3a11938a9da02114657cbed9069c9))


### Miscellaneous

* add commit-msg hook and ai-attribution rule ([13fb8e1](https://github.com/bluegardenproject/stac-man/commit/13fb8e14d894e69cc1dcfcef0d7ae2ca093613a8))
* initial scaffold ([81f7ef1](https://github.com/bluegardenproject/stac-man/commit/81f7ef10ff11a6b3b9d68f49e5d7458f9c2864bf))
* **release:** set baseline version to 0.0.1 ([3a21b00](https://github.com/bluegardenproject/stac-man/commit/3a21b00466a26920d3e178a8a27d0537df93035f))

## Changelog
