# gNSI Credentialz Tests

## Summary
Test gNSI Credentialz API behaviors.

## Request Examples
These example gNSI credentialz requests are examples that can be used with cases
below.

### Configure a testuser and password

```
stream.Send(
    RotateAccountCredentialsRequest {
        password: PasswordRequest {
            accounts: Account {
                account: "testuser",
                password: Password {
                    value: {
                        plaintext: "i$V5^6IhD*tZ#eg1G@v3xdVZrQwj",
                    }
                },
                version: "v1.4",
                created_on: 3214451134,
            }
        }
    }
)

resp := stream.Receive()
```

### Set the TrustedUserCaKeys

Note: You will need to set the contents of the ssh_ca_public_keys message to the
contents of your CA public key file.

```
stream.Send(
    RotateHostParametersRequest {
        ssh_ca_public_key: CaPublicKeyRequest {
            ssh_ca_public_keys: "A....=",
            version: "v1.0",
            created_on: 3214451134,
        }
    }
)

resp := stream.Receive()
```

### Disallow passwords for SSH

```
stream.Send(
    RotateHostParametersRequest {
        authentication_allowed: AllowedAuthenticationRequest {
            authentication_types: AuthenticationType {
                AuthenticationType_PUBKEY.Enum(),
            }
        }
    }
)
```

### Populate Authorized Principals

```
stream.Send(
    RotateAccountCredentialsRequest {
        user: AuthorizedUsersRequest {
            policies: UserPolicy {
                account: "testuser",
                authorized_principals: SshAuthorizedPrincipal {
                    authorized_user: "principal_name",
                },
                version: "v1.4",
                created_on: 3214451134,
            }
        }
    }
)

resp := stream.Receive()
```

### Populate Authorized Principals
Note: Key contents must be the public key from your generated user keypair.

```
stream.Send(
    RotateAccountCredentialsRequest {
        credential: AuthorizedKeysRequest {
            credentials: AccountCredentials {
                account: "testuser",
                authorized_keys: AuthorizedKey {
                    authorized_key: "A....=",
                },
                version: "v1.4",
                created_on: 3214451134,
            }
        }
    }
)

resp := stream.Receive()
```

### Set the Host Certificate

Note: You will need to set the contents of the host certificate generated from
your test setup.

```
stream.Send(
    RotateHostParametersRequest {
        server_keys: ServerKeysRequest {
            auth_artifacts: []AuthenticationArtifacts{
                certificate: []bytes("...."),
            },
            version: "v1.0",
            created_on: 3214451134,
        }
    }
)

resp := stream.Receive()
```

## Tests

### Credentialz-1, Password console login

#### Setup

* Set a username of "testuser" using gnsi.Credentialz
* Set a password of "i$V5^6IhD*tZ#eg1G@v3xdVZrQwj" (see RotateAccountCredentials, PasswordRequest, plaintext)
* Connect to the console


#### Pass case
* Provide correct username/password on console.
  * Authentication must result in success with a prompt.
  * There must be accounting for the login which includes the `testuser`
    username.

#### Fail case 1
* Provide incorrect username and correct password.
  * Authentication must fail.

#### Fail case 2
* Provide incorrect password, but correct username.
  * Authentication must fail.

### Credentialz-2, SSH pasword login disallowed

#### Setup
* Set a username of `testuser`
* Set a password of `i$V5^6IhD*tZ#eg1G@v3xdVZrQwj` (see
  RotateAccountCredentials, PasswordRequest, plaintext)
* Ensure that AAA (TACACS+/Radius) authentication is not configured.
* Create a ssh CA keypair with `ssh-keygen -f /tmp/ca`.
* Create a user keypair with `ssh-keygen -t ed25519`
* Sign the user public key into a certificate using the CA using `ssh-keygen -s
  /tmp/ca -I testuser -n principal_name -V +52w ~/.ssh/id_ed25519.pub`. You will
  find your certificate in `~/.ssh` ending in `-cert.pub`.
* Set the device TrustedUserCAKeys (see RotateHostParameters
  ssh_ca_public_key) with the CA public key, found in `/tmp/ca.pub`.
* Set authentication types to only permit PUBKEY (see
  AllowedAuthenticationRequest).
* Set authorized_users for `testuser` with a principal of `principal_name`.

#### Pass case
* Attempt an ssh authentication using the username (ssh testuser@DUT) and password.
  * Authentication must fail.
* Attempt password authentication on the console.
  * Authentication must result in success with a prompt.
* Attempt certificate authentication over ssh, `ssh testuser@DUT`.
  * Use the ssh user certificate with a signature verifiable by a
    TrustedUserCAKey public key created above.
  * Authentication must succeed.
  * Accounting, using gnsi Accounting must set the identity string (see
    acct.proto AuthDetail message) to equal the principal (principal_name)
    from the certificate rather than the system role (testuser).

### Credentialz-3, Host Certificate

#### Setup
* Create a ssh CA keypair with `ssh-keygen -f /tmp/ca`.
* Fetch the ssh server's host public key.
* Sign the public key from the previous step into a host certificate using the
  CA key `ssh-keygen -s /tmp/ca -I dut -h -n dut.test.com -V +52w
  /location/of/host/public_key.pub`
* Add the certificate to the server (see RotateHostParameters,
  AuthenticationArtifacts, certificate)

#### Pass case
* ssh to the server.
  * You must receive the host certificate signed by your CA.

### Credentialz-4, SSH Public Key Authentication

#### Setup
* Create a user ssh CA keypair with `ssh-keygen`. No arguments are required and
  the keys will be put in `~/.ssh/`.
* Create a username on the ssh server and add the public key (see
  RotateAccountCredentials AuthorizedKeysRequest).

#### Pass case
* Attempt to ssh into the server with the username, presenting the ssh key.
  * Authentication must succeed.

#### Fail case
* Remove the user ssh key (by sending an AuthorizedKeysRequest with no
  authorized_keys message.
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
  * Accounting, using gnsi Accounting must set the identity string (see
    acct.proto AuthDetail message) to equal the principal (principal_name)
    from the certificate rather than the system role (testuser).

#### Fail case
* Remove the certificate with the HIBA grant (or wait for expiration)
* Create an ssh certificate with no grants.
* Log into the server as "testuser" with this certificate
  * Authentication must fail

