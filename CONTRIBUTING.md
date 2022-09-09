# Contributing the Openconfig Feature Profiles

Thank you for your interest in contributing to OpenConfig feature profiles.

## Rationale

See the [README](README.md) for an explanation of what OpenConfig feature
profiles are and why we have them.

## Ways to Contribute

OpenConfig prefers contributions in the form of code, documentation and even bug
reporting. If you wish to discuss the suitability or approach for a change this
can be done with an issue in the
[OpenConfig feature profiles GitHub](https://github.com/openconfig/featureprofiles/issues).

## Contributor License Agreement

All contributions to OpenConfig feature profiles MUST be Apache 2.0 licensed.
The
[Google contributor license agreement (CLA)](https://cla.developers.google.com/),
MUST be signed for any contribution to be merged into the repository.

The CLA is used to ensure that the rights to use the contribution are well
understood by the OpenConfig community. Since copyright over each contribution
is assigned to its authors, code comments should reflect the contribution made,
and the copyright holder. No code will be reviewed if the license is not
explicitly stated, or the CLA has not been signed.

Note that we use the Google CLA because the OpenConfig project is
[administered and maintained by Google](https://opensource.google.com/docs/cla/#why),
not to ascribe any specific rights to a single OpenConfig contributor.

## Style Guides

Code and Test Plan contributions should follow the
[Code Style Guide](docs/code-style.md) and
[Test Plan Style Guide](docs/testplan-style.md). The purpose for having these
style guides is to ensure that code and test plans in this repository are
consistent even though they are contributed by many authors. These authors may
be from different organizations and have different areas of expertise. Having a
consistent style ensures that we can communicate effectively using the code and
test plan as a way to:

1.  Invest in shared engineering knowledge, and
2.  Propagate consistent standards and best practices.

Pull request reviewers are expected to request or suggest changes to the pull
request based on these style guides.

## Pull Requests

To contribute a pull request:

1.  Make a fork of the
    [openconfig/featureprofiles](https://github.com/openconfig/featureprofiles)
    repo and make the desired changes, following the
    [GitHub Quickstart](https://docs.github.com/en/get-started/quickstart)
    guide.

    *   New contributions should be in the feature/experimental directory.

1.  When opening a pull request, use a descriptive title and detail. See
    [Pull Request Title](#pull-request-title) below.

    *   Pull requests should be kept small, ideally under 500 lines. Small
        changes allow detailed discussions of the additions that are being made
        to the model, whilst also ensuring that course-corrections can be made
        early in the process. If a PR is growing to more than 500 lines, it may
        need to be broken into multiple smaller PRs.

1.  The pull request should include both the suggested feature profile textproto
    additions, as well as any relevant additions to tests. Tests should be
    written in Go using the [ONDATRA](https://github.com/openconfig/ondatra)
    framework.

1.  The automated CI running against each pull request will check the pull
    request for compliance. The author should resolve any issues found by CI.

1.  One or more peers in the community may review the pull request.

1.  A feature profile repository maintainer will be reponsible for a final
    review and approval. Only a feature repository maintainer can merge a pull
    request to the main branch.

The aim of this process is maintain the model quality and approach that
OpenConfig working group has strived for since its inception in 2014. Thank you
for your contributions!

### Pull Request Title

Pull request title should start with the scope (i.e. section in the test plan).

```
RT-5.1 singleton configuration and telemetry, no traffic test yet
```

The description may add more details as desired to benefit the reviewers. The
preferred format is:

```
    * (M) yang/keychain/openconfig-keychain.yang
      - Add a typedef for referencing a keychain from other modules.
    * (M) yang/isis/*
      - Fix support for hello-authentication to allow for references to a
        specific keychain as defined in the keychain model.
      - Fix support for authentication of *SNP packets, referencing a
        keychain that can be used to auth these packets.
      - move IS-IS model to openconfig-inet-types rather than ietf-inet-types.
```
