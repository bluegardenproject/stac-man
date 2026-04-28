# Changelog

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
