module github.com/hyperledger/fabric/extensions

replace github.com/hyperledger/fabric => ../.

replace github.com/hyperledger/fabric/extensions => ./

require (
	github.com/hyperledger/fabric v1.4.0
	github.com/spf13/viper v0.0.0-20150908122457-1967d93db724
	github.com/stretchr/testify v1.3.0
)
