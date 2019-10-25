module github.com/hyperledger/fabric

require (
	code.cloudfoundry.org/clock v0.0.0-20180518195852-02e53af36e6c
	github.com/Knetic/govaluate v3.0.0+incompatible
	github.com/Shopify/sarama v1.19.0
	github.com/Shopify/toxiproxy v2.1.4+incompatible // indirect
	github.com/VividCortex/gohistogram v1.0.0 // indirect
	github.com/coreos/pkg v0.0.0-20180108230652-97fdf19511ea // indirect
	github.com/davecgh/go-spew v1.1.1
	github.com/eapache/go-resiliency v1.1.0 // indirect
	github.com/eapache/go-xerial-snappy v0.0.0-20180814174437-776d5712da21 // indirect
	github.com/eapache/queue v1.1.0 // indirect
	github.com/fsouza/go-dockerclient v1.4.0
	github.com/go-kit/kit v0.8.0
	github.com/gogo/protobuf v1.2.1
	github.com/golang/protobuf v1.3.1
	github.com/gorilla/handlers v1.4.0
	github.com/gorilla/mux v1.7.1
	github.com/grpc-ecosystem/go-grpc-middleware v1.0.0
	github.com/hashicorp/go-version v1.2.0
	github.com/hyperledger/fabric-amcl v0.0.0-20181230093703-5ccba6eab8d6
	github.com/hyperledger/fabric-lib-go v1.0.0
	github.com/hyperledger/fabric/extensions v0.0.0
	github.com/kr/pretty v0.1.0
	github.com/magiconair/properties v1.8.1 // indirect
	github.com/mattn/go-runewidth v0.0.4 // indirect
	github.com/miekg/pkcs11 v0.0.0-20190429190417-a667d056470f
	github.com/mitchellh/mapstructure v1.1.2
	github.com/onsi/ginkgo v1.8.0
	github.com/onsi/gomega v1.5.0
	github.com/op/go-logging v0.0.0-20160315200505-970db520ece7
	github.com/pierrec/lz4 v0.0.0-20190501090746-d705d4371bfc // indirect
	github.com/pkg/errors v0.8.1
	github.com/prometheus/client_golang v0.9.3
	github.com/prometheus/procfs v0.0.0-20190521135221-be78308d8a4f // indirect
	github.com/rcrowley/go-metrics v0.0.0-20181016184325-3113b8401b8a
	github.com/spf13/cast v1.2.0 // indirect
	github.com/spf13/cobra v0.0.3
	github.com/spf13/jwalterweatherman v1.0.0 // indirect
	github.com/spf13/oldviper v0.0.0
	github.com/spf13/pflag v1.0.3
	github.com/stretchr/testify v1.3.0
	github.com/sykesm/zap-logfmt v0.0.2
	github.com/syndtr/goleveldb v0.0.0-20190318030020-c3a204f8e965
	github.com/tedsuo/ifrit v0.0.0-20180802180643-bea94bb476cc
	github.com/willf/bitset v1.1.10
	go.etcd.io/etcd v0.0.0-20181228115726-23731bf9ba55
	go.uber.org/zap v1.10.0
	golang.org/x/crypto v0.0.0-20190513172903-22d7a77e9e5f
	golang.org/x/net v0.0.0-20190520210107-018c4d40a106
	golang.org/x/sys v0.0.0-20190523142557-0e01d883c5c5 // indirect
	golang.org/x/tools v0.0.0-20190313210603-aa82965741a9
	google.golang.org/appengine v1.4.0 // indirect
	google.golang.org/genproto v0.0.0-20190516172635-bb713bdc0e52 // indirect
	google.golang.org/grpc v1.20.1
	gopkg.in/alecthomas/kingpin.v2 v2.2.6
	gopkg.in/cheggaaa/pb.v1 v1.0.28
	gopkg.in/yaml.v2 v2.2.2

)

replace github.com/docker/libnetwork => github.com/docker/libnetwork v0.0.0-20180608203834-19279f049241

replace github.com/docker/docker => github.com/docker/docker v0.0.0-20180827131323-0c5f8d2b9b23

replace github.com/hyperledger/fabric/extensions => ./extensions

replace github.com/spf13/oldviper => github.com/spf13/viper v0.0.0-20150908122457-1967d93db724
