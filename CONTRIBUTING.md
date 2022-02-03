# Contributing the Openconfig feature profiles

Thank you for your interest in contributing to OpenConfig feature profiles.  

## Rationale
See the [README](README.md) for an explanation of what OpenConfig feature 
profiles are and why we have them. 

## Contributing to OpenConfig feature profiles

OpenConfig prefers contributions in the form of code, documentation and
even bug reporting. If you wish to discuss the suitability or approach 
for a change this can be done with an issue in the 
[OpenConfig feature profiles GitHub](https://github.com/openconfig/featureprofiles/issues). 

## Contributor License Agreement
All contributions to OpenConfig feature profiles MUST be Apache 2.0 licensed. 
The [Google contributor license agreement (CLA)](https://cla.developers.google.com/), 
MUST be signed for any contribution to be merged into the repository. 

The CLA is used to ensure that the rights to use the contribution are well
understood by the OpenConfig community. Since copyright over each contribution
is assigned to its authors, code comments should reflect the contribution 
made, and the copyright holder. No code will be reviewed if the license is
not explicitly stated, or the CLA has not been signed.

Note that we use the Google CLA because the OpenConfig project is [administered
and maintained by Google](https://opensource.google.com/docs/cla/#why), not to
ascribe any specific rights to a single OpenConfig contributor.

## Make a contribution
To make a contribution to OpenConfig featureprofiles:

1. Open a pull request in the
 [openconfig/featureprofiles](https://github.com/openconfig/featureprofiles) 
 repo. A brief description of the proposed addition along with references to 
 any discussion issues should be included.
    * Pull requests should be kept small. An ideal change is less than 500 lines. 
     Small changes allow detailed discussions of the additions that are
     being made to the model, whilst also ensuring that course-corrections can be
     made early in the process. If a test is growing to more than 500 lines, it
     may need to be broken into multiple smaller tests.

1. The pull request should include both the suggested feature profile textproto 
 additions, as well as any relevant additions to tests. Tests should be written
 in golang using the [ONDATRA](https://github.com/openconfig/ondatra) framework.

1. The automated CI running against each pull request will check the pull
 request for compliance.  The author should resolve any issues found by CI.

1. One or more peers in the community may review the pull request.   

1. A feature profile repository maintainer will be reponsible for a final review
and approval.  Only a feature repository maintainer can merge a pull request to 
the main branch.
  
The aim of this process is maintain the model quality and approach that OpenConfig 
working group has strived for since its inception in 2014. Thank you for your contributions!
