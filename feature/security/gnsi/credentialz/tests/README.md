# gNSI Credentialz Tests

## Summary
Test gNSI Credentialz API behaviors.


## Tests

### Credentialz-1, Password console login

#### Setup

* Set a username of "testuser"
* Set a password of "i$V5^6IhD*tZ#eg1G@v3xdVZrQwj" (see RotateAccountCredentials, PasswordRequest, plaintext)
* Connect to the console

#### Pass case
* Provide correct username/password on console.
  * Authentication must result in success with a prompt.

#### Fail case 1
* Provide incorrect username and correct password.
  * Authentication must fail.

#### Fail case 2
* Provide incorrect password, but correct username.
  * Authentication must fail.

### Credentialz-2, SSH pasword login disallowed

#### Setup
* Set a username of "testuser"
* Set a password of "i$V5^6IhD*tZ#eg1G@v3xdVZrQwj" (see
  RotateAccountCredentials, PasswordRequest, plaintext)
* Create a ssh CA keypair with `ssh-keygen`.
* Create a user keypair with `ssh-keygen`.
* Sign the user public key into a certificate using the CA.
* Set up the device with a TrustedUserCAKey (see RotateHostParameters
  ssh_ca_public_key) with the CA public key.
* Set authentication types to only permit PUBKEY (see
  AllowedAuthenticationRequest).

#### Pass case
* Attempt an ssh authentication using the username and password.
  * Authentication must fail.
* Attempt password authentication on the console.
  * Authentication must result in success with a prompt.
* Attempt certificate authentication over ssh.
  * Use an ssh user certificate with a signature verifiable by a
    TrustedUserCAKey public key.
  * Authentication must succeed.

### Credentialz-3, Client authenticates server

#### Setup
* Create a ssh CA keypair with `ssh-keygen` in /tmp/hostca/ca.
* Ask the ssh server to generate a host keypair (see RotateHostParameters,
  GenerateKeysRequest)
* Sign the public key returned from the previous step into a host certificate
  using the CA key.
* Add the certificate to the server (see RotateHostParameters,
  AuthenticationArtifacts, certificate)
* On the client, set the certificate authority `echo "@cert-authority * $(cat /tmp/hostca/ca.pub)"
  >>~/.ssh/known_hosts`

#### Pass case
* Ensure the @cert-authority is the only entry in your `known_hosts` file
* ssh to the server.
  * You should not be prompted to verify a host fingerprint.

#### Fail case
* Ensure the @cert-authority is the only entry in your `known_hosts` file
* Modify the @cert-authority line in known_hosts, changing a couple characters
  in the fingerprint.
* ssh to the server.
  * You should fail to authenticate the host certificate and be presented with a
    host key to manually verify.

### Credentialz-4, SSH Public Key Authentication

#### Setup
* Create a user ssh CA keypair with `ssh-keygen`.
* Create a username on the ssh server and add the public key (see
  RotateAccountCredentials AuthorizedKeysRequest).

#### Pass case
* Attempt to ssh into the server with the username, presenting the ssh key.
  * Authentication must succeed.

#### Fail case
* Remove the user ssh key.
* Attempt to ssh into the server with the username.
* Public key authentication should fail.

### Credentialz-5, HIBA Authentication

#### Setup
* Set a username of "testuser"
* Follow the instructions for setting up a [HIBA
  CA](https://github.com/google/hiba/blob/main/CA.md)
* Sign the user public key into a certificate using the CA with a "shell" grant.
* Set up the device with a TrustedUserCAKey (see RotateHostParameters
  ssh_ca_public_key) with the CA public key.
* Set authentication types to only permit PUBKEY (see
  AllowedAuthenticationRequest).
* Set the AuthorizedPrincipalsCommand by setting the tool to `TOOL_HIBA_DEFAULT`
  (see RotateHostParameters, AuthorizedPrincipalCheckRequest)

#### Pass case
* Log into the server as "testuser" using the client certificate.
  * Authentication must succeed.

#### Fail case
* Remove the certificate with the HIBA grant (or wait for expiration)
* Create an ssh certificate with no grants.
* Log into the server as "testuser" with this certificate
  * Authentication must fail

