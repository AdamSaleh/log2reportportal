# log2reportportal

<div align="center">

[![Build Status](https://img.shields.io/github/checks-status/AdamSaleh/log2reportportal/main?color=black&style=for-the-badge&logo=github)][github-actions]
[![Code Coverage](https://img.shields.io/codecov/c/github/AdamSaleh/log2reportportal?color=blue&logo=codecov&style=for-the-badge)][github-actions-tests]
[![Security: bandit](https://img.shields.io/badge/Security-GoSec-lightgrey?style=for-the-badge&logo=springsecurity)](https://github.com/securego/gosec)
[![Dependencies Status](https://img.shields.io/badge/Dependencies-Up%20to%20Date-brightgreen?style=for-the-badge&logo=dependabot)][dependabot-pulls]
[![Semantic Versioning](https://img.shields.io/badge/versioning-semantic-black?style=for-the-badge&logo=semver)][github-releases]
[![Pre-Commit Enabled](https://img.shields.io/badge/Pre--Commit-Enabled-blue?style=for-the-badge&logo=pre-commit)][precommit-config]
[![License](https://img.shields.io/github/license/AdamSaleh/log2reportportal?color=red&style=for-the-badge)][project-license]
[![Go v1.18](https://img.shields.io/badge/Go-%20v1.18-black?style=for-the-badge&logo=go)][gomod-file]

A cli tool to upload testlogs as launches to reportportal

Takes log on stdin, and uploads the test-results to a specific project as a new launch, for example:

```
export RP_TOKEN=<token>
cat test_data/argocd-e2e-186_last.log | log2reportportal -name launch20240101 -project gitops-adhoc -url https://reportportal-gitops-qe.apps.ocp-c1.prod.psi.redhat.com -skipTs
```

should then appear as a new launch in https://reportportal-gitops-qe.apps.ocp-c1.prod.psi.redhat.com

</div>


## Initial Setup

This section is intended to help developers and contributors get a working copy of
`log2reportportal` on their end

<details>
<summary>
    1. Clone this repository
</summary><br>

```sh
git clone https://github.com/AdamSaleh/log2reportportal
cd log2reportportal
```
</details>

<details>
<summary>
    2. Install `golangci-lint`
</summary><br>

Install `golangci-lint` from the [official website][golangci-install] for your OS
</details>
<br>



### Installing dependencies

To install all dependencies associated with `log2reportportal`, run the
command

```sh
make install
```

### Using Code Formatters

Code formatters format your code to match pre-decided conventions. To run automated code
formatters, use the Makefile command

```sh
make codestyle
```

### Using Code Linters

Linters are tools that analyze source code for possible errors. This includes typos,
code formatting, syntax errors, calls to deprecated functions, potential security
vulnerabilities, and more!

To run pre-configured linters, use the command

```sh
make lint
```

### Running the Test-Suite

The *`test-suite`* is simply a wrapper to run linters, stylecheckers and **all** tests
at once!

To run the test-suite, use the command

```sh
make test-suite
```

In simpler terms, running the test-suite is a combination of running [linters](#using-code-linters)
and [all tests](#running-tests) one after the other!
<br>

## Credits

<div align="center"><br>

`log2reportportal` is powered by a template generated using [`go-template`][go-template-link]

[![go-template](https://img.shields.io/badge/go--template-black?style=for-the-badge&logo=go)][go-template-link]
