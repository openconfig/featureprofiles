# Contributing the Openconfig feature profiles

Thank you for your interest in contributing to OpenConfig feature profiles.  

## Rationale
See the [README](README.md) for an explanation of what OpenConfig feature 
profiles are and why we have them. Openconfig feature profiles are part 
of the openconfig project and managed by the same policies as the other 
repositories in the project.

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
     made early in the process. In some cases, changes larger than 500 lines may
     be unavoidable - these should be rare, and generally only be the case when
     entirely new modules are being added to the model. In this case, it is very
     likely an issue should have been created to discuss the addition prior to
     code review.
    * When the pull request adds a new feature that is supported across vendors,
     best practice is to include links to public-facing documentation showing
     the implementation of the feature within the change description. This
     simplifies the process of reviewing differences and the chosen abstractions
     (if any are used).

1. The pull request should include both the suggested feature profile textproto 
 additions, as well as any relevant additions to tests. Tests should be written
 in golang using the [ONDATRA](https://github.com/openconfig/ondatra) framework.

1. The automated CI running against each pull request will check the pull
 request for compliance.  The author should resolve any issues found by CI.

1. One or more peers in the community may review the pull request.   

1. A feature profile repository maintainer will be reponsible for a final review
and approval.  Only a feature repository maintainer can merge a pull request to 
the main branch.
  
The aim of this process is maintain the model quality and approach that Openconfig 
working group has strived for since its inception in 2014. Thank you for your contributions!
