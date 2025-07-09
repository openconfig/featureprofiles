# Credentialz-1: Password console login

## Summary

Test that Credentialz properly creates users and the associated password and that the DUT handles
authentication of those users properly.


## Procedure

* Set a username of `testuser` with a password having following restrictions:
  * Must be 24-32 characters long.
  * Must use 4 of the 5 character classes ([a-z], [A-Z], [0-9], [!@#$%^&*(){}[]\|:;'"], [ ]).
* Perform the following tests and assert the expected result:
  * Case 1: Success
    * Authenticate with the `testuser` username and password created in the first step above.
    * Authentication must result in success with a prompt.
  * Case 2: Failure
    * Authenticate with the `testuser` username and an *incorrect* password of `password`
    * Assert that authentication has failed
  * Case 3: Failure
    * Authenticate with the invalid  username `username` and a valid (for a different username)
      password created in the first step above.
    * Assert that authentication has failed


## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths intended to be covered by this test. OC paths used for test setup are not listed here.

```yaml
rpcs:
  gnsi:
    credentialz.v1.Credentialz.RotateAccountCredentials:
```


## Minimum DUT platform requirement

N/A
