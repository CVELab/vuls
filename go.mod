module github.com/cvelab/vuls

go 1.20

require (
	github.com/asaskevich/govalidator v0.0.0-20230301143203-a9d515a09cc2
	github.com/google/subcommands v1.2.0
	go.etcd.io/bbolt v1.3.7
	golang.org/x/xerrors v0.0.0-20220907171357-04be3eba64a2
)

require (
	github.com/stretchr/testify v1.8.3 // indirect
	golang.org/x/sys v0.8.0 // indirect
)

// See https://github.com/moby/moby/issues/42939#issuecomment-1114255529
replace github.com/docker/docker => github.com/docker/docker v20.10.3-0.20220224222438-c78f6963a1c0+incompatible
