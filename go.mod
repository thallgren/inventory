module github.com/puppetlabs/inventory

go 1.13

require (
	github.com/gofrs/flock v0.7.1
	github.com/jirenius/go-res v0.2.0
	github.com/lyraproj/dgo v0.3.2
	github.com/lyraproj/dgoyaml v0.3.1
	github.com/nats-io/nats-server/v2 v2.1.2 // indirect
	github.com/sirupsen/logrus v1.4.2
	golang.org/x/crypto v0.0.0-20200115085410-6d4e4cb37c7d // indirect
)

replace github.com/lyraproj/dgo => github.com/thallgren/dgo v0.0.0-20200202122546-ee785400833b
