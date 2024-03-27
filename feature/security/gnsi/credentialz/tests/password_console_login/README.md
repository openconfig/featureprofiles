# Credentialz-1: Password console login

## Summary

Test that Credentialz properly creates users and the associated password and that the DUT handles
authentication of those users properly.


## Procedure

* Set a username of `testuser` with a password of `i$V5^6IhD*tZ#eg1G@v3xdVZrQwj` using gnsi.Credentialz
* Perform the following tests and assert the expected result:
  * Case 1: Success
    * Authenticate with the `testuser` username and password `i$V5^6IhD*tZ#eg1G@v3xdVZrQwj`
    * Assert that authentication was successful and some command can be executed. Note: we 
      simply send a command here rather than assert we "see" a prompt, we do this because this 
      doesn't require an interactive shell, and is an easy way to validate we are authenticated.
  * Case 2: Failure
    * Authenticate with the `testuser` username and an *incorrect* password of `password`
    * Assert that authentication has failed
  * Case 3: Failure
    * Authenticate with the invalid  username `username` and a valid (for a different username) 
      password of `i$V5^6IhD*tZ#eg1G@v3xdVZrQwj`
    * Assert that authentication has failed


## Config Parameter coverage

* /gnsi/credz


## Telemetry Parameter coverage

N/A


## Protocol/RPC Parameter coverage

N/A


## Minimum DUT platform requirement

N/A
