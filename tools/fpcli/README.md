fpcli is an OpenConfig helper CLI for featureprofile-related use cases.

For example, you can use it to show what RPCs exist for a particular OpenConfig
protocol:

### Example

```bash
go install ./tools/fpcli
fpcli show rpcs gnoi -d tmp
```

Output:

```
gnoi.bgp.BGP.ClearBGPNeighbor gnoi.bootconfig.BootConfig.GetBootConfig
gnoi.bootconfig.BootConfig.SetBootConfig
gnoi.certificate.CertificateManagement.CanGenerateCSR
gnoi.certificate.CertificateManagement.GenerateCSR
...
```
