module github.com/hyperledger/fabric/extensions

replace github.com/hyperledger/fabric => ../.

replace github.com/hyperledger/fabric/extensions => ./

require (
	github.com/hyperledger/fabric v2.0.0-alpha+incompatible
	github.com/hyperledger/fabric-protos-go v0.0.0-20191121202242-f5500d5e3e85
	github.com/pkg/errors v0.8.1
	github.com/spf13/viper v1.3.2
	github.com/stretchr/testify v1.4.0
)

replace github.com/hyperledger/fabric-protos-go => github.com/trustbloc/fabric-protos-go-ext v0.1.1-0.20191126151100-5a61374c2e1b

replace github.com/spf13/viper => github.com/spf13/viper v0.0.0-20150908122457-1967d93db724

go 1.13
