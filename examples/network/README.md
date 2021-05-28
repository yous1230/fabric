## Build Your First Network (BYFN)

The directions for using this are documented in the Hyperledger Fabric
["Build Your First Network"](http://hyperledger-fabric.readthedocs.io/en/latest/build_network.html) tutorial.

*NOTE:* After navigating to the documentation, choose the documentation version that matches your version of Fabric

### rebuild tools
```
make cryptogen

make configtxgen

export PATH=$GOPATH/src/github.com/hyperledger/fabric/.build/bin:$PATH

```

### start guomi network
```
./byfn.sh up -g
```