module github.com/hyperledger/fabric

go 1.13

require (
	code.cloudfoundry.org/clock v1.0.0
	github.com/DataDog/zstd v1.4.0 // indirect
	github.com/Knetic/govaluate v3.0.0+incompatible
	github.com/Shopify/sarama v1.20.1
	github.com/Shopify/toxiproxy v2.1.4+incompatible // indirect
	github.com/VictoriaMetrics/fastcache v1.4.6
	github.com/VividCortex/gohistogram v1.0.0 // indirect
	github.com/containerd/continuity v0.0.0-20190426062206-aaeac12a7ffc // indirect
	github.com/coreos/go-systemd v0.0.0-20190620071333-e64a0ec8b42a // indirect
	github.com/coreos/pkg v0.0.0-20180108230652-97fdf19511ea // indirect
	github.com/davecgh/go-spew v1.1.1
	github.com/docker/docker v17.12.0-ce-rc1.0.20190628135806-70f67c6240bb+incompatible // indirect
	github.com/dustin/go-humanize v1.0.0 // indirect
	github.com/eapache/go-resiliency v1.2.0 // indirect
	github.com/eapache/go-xerial-snappy v0.0.0-20180814174437-776d5712da21 // indirect
	github.com/eapache/queue v1.1.0 // indirect
	github.com/frankban/quicktest v1.10.0 // indirect
	github.com/fsouza/go-dockerclient v1.4.1
	github.com/go-kit/kit v0.8.0
	github.com/golang/protobuf v1.3.3
	github.com/gorilla/handlers v1.4.0
	github.com/gorilla/mux v1.7.2
	github.com/grpc-ecosystem/go-grpc-middleware v1.1.0
	github.com/hashicorp/go-version v1.2.0
	github.com/hyperledger/fabric-amcl v0.0.0-20200128223036-d1aa2665426a
	github.com/hyperledger/fabric-chaincode-go v0.0.0-20200128192331-2d899240a7ed
	github.com/hyperledger/fabric-lib-go v1.0.0
	github.com/hyperledger/fabric-protos-go v0.0.0-20200326212758-d7d9b8e1fcde
	github.com/hyperledger/fabric/extensions v0.0.0-00010101000000-000000000000
	github.com/konsorten/go-windows-terminal-sequences v1.0.2 // indirect
	github.com/kr/pretty v0.2.0
	github.com/magiconair/properties v1.8.1 // indirect
	github.com/mattn/go-runewidth v0.0.4 // indirect
	github.com/miekg/pkcs11 v1.0.3
	github.com/mitchellh/mapstructure v1.1.2
	github.com/onsi/ginkgo v1.8.0
	github.com/onsi/gomega v1.9.0
	github.com/opencontainers/runc v1.0.0-rc8 // indirect
	github.com/pierrec/lz4 v2.5.2+incompatible // indirect
	github.com/pkg/errors v0.8.1
	github.com/prometheus/client_golang v1.1.0
	github.com/rcrowley/go-metrics v0.0.0-20181016184325-3113b8401b8a
	github.com/spf13/cobra v0.0.5
	github.com/spf13/jwalterweatherman v1.1.0 // indirect
	github.com/spf13/pflag v1.0.3
	github.com/spf13/viper2015 v1.3.2
	github.com/stretchr/testify v1.5.1
	github.com/sykesm/zap-logfmt v0.0.2
	github.com/syndtr/goleveldb v1.0.1-0.20190625010220-02440ea7a285
	github.com/tedsuo/ifrit v0.0.0-20180802180643-bea94bb476cc
	github.com/willf/bitset v1.1.10
	go.etcd.io/etcd v0.5.0-alpha.5.0.20181228115726-23731bf9ba55
	go.uber.org/zap v1.14.1
	golang.org/x/crypto v0.0.0-20200221231518-2aa609cf4a9d
	golang.org/x/mod v0.3.0 // indirect
	golang.org/x/text v0.3.2 // indirect
	golang.org/x/tools v0.0.0-20200626171337-aa94e735be7f
	google.golang.org/grpc v1.28.0
	gopkg.in/alecthomas/kingpin.v2 v2.2.6
	gopkg.in/cheggaaa/pb.v1 v1.0.28
	gopkg.in/yaml.v2 v2.2.4
)

replace gopkg.in/fsnotify.v1 v1.4.7 => gopkg.in/fsnotify/fsnotify.v1 v1.4.7

replace github.com/docker/libnetwork => github.com/docker/libnetwork v0.0.0-20180608203834-19279f049241

replace github.com/docker/docker => github.com/docker/docker v0.0.0-20180827131323-0c5f8d2b9b23

replace github.com/hyperledger/fabric/extensions => ./extensions

replace github.com/hyperledger/fabric-protos-go => github.com/trustbloc/fabric-protos-go-ext v0.1.4-0.20200626180529-18936b36feca

replace github.com/spf13/viper2015 => github.com/spf13/viper v0.0.0-20150908122457-1967d93db724
