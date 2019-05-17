# TrustBloc fabric-mod

fabric-mod defines compile-time extension points for [Hyperledger Fabric](https://github.com/hyperledger/fabric) by leveraging [Go Modules](https://github.com/golang/go/wiki/Modules). All extensions points are grouped under the [`github.com/hyperledger/fabric/extensions`](./extensions) module.

```
# make sure you clone into hyperledger/fabric
git clone https://github.com/trustbloc/fabric-mod.git $GOPATH/src/github.com/hyperledger/fabric

cd fabric

make all
```

## Motivation

 Hyperledger Fabric does perform and scale nicely, given the right compute and networking infrastructure. However, it can also yields results that are inadequate for individual application needs. Therefore, Fabric-mod came into existence to enhance the code performance by introducing extensions. Hyperledger Fabric enhancements can be located under extensions folder in the source code.
 
## Contributing
Thank you for your interest in contributing. Please see our [community contribution guidelines](https://github.com/trustbloc/community/blob/master/CONTRIBUTING.md) for more information.

## License

fabric-mod is made available under the Apache License, Version 2.0 (Apache-2.0), located in the [LICENSE](LICENSE) file.

