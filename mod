module github.com/hyperledger/fabric

go 1.15

require (
	code.cloudfoundry.org/clock v1.0.0
	github.com/Azure/go-ansiterm v0.0.0-20170929234023-d6e3b3328b78
	github.com/BurntSushi/toml v0.3.1
	github.com/DataDog/zstd v1.4.8
	github.com/Knetic/govaluate v3.0.1-0.20171022003610-9aa49832a739+incompatible
	github.com/Microsoft/go-winio v0.4.16
	github.com/Nvveen/Gotty v0.0.0-20120604004816-cd527374f1e5
	github.com/Shopify/sarama v1.27.2
	github.com/SmartBFT-Go/consensus v0.0.0-20201014162554-738523406382
	github.com/SmartBFT-Go/randomcommittees v0.0.0-20210126151617-8b5a6b12ab51
	github.com/alecthomas/template v0.0.0-20190718012654-fb15b899a751
	github.com/alecthomas/units v0.0.0-20201120081800-1786d5ef83d4
	github.com/beorn7/perks v1.0.1
	github.com/containerd/continuity v0.0.0-20201208142359-180525291bb7
	github.com/coreos/go-systemd v0.0.0-20190321100706-95778dfbb74e
	github.com/coreos/pkg v0.0.0-20180928190104-399ea9e2e55f
	github.com/davecgh/go-spew v1.1.1
	github.com/docker/docker v0.7.3-0.20180827131323-0c5f8d2b9b23
	github.com/docker/go-connections v0.4.0
	github.com/docker/go-units v0.3.3
	github.com/docker/libnetwork v0.8.0-dev.2.0.20180608203834-19279f049241
	github.com/dustin/go-humanize v1.0.0
	github.com/eapache/go-resiliency v1.2.0
	github.com/eapache/go-xerial-snappy v0.0.0-20180814174437-776d5712da21
	github.com/eapache/queue v1.1.0
	github.com/fsouza/go-dockerclient v1.3.0
	github.com/go-kit/kit v0.10.0
	github.com/go-logfmt/logfmt v0.5.0
	github.com/go-stack/stack v1.8.0
	github.com/gogo/protobuf v1.3.2
	github.com/golang/lint v0.0.0-20180702182130-06c8688daad7 // indirect
	github.com/golang/protobuf v1.4.3
	github.com/golang/snappy v0.0.2
	github.com/gorilla/handlers v1.5.1
	github.com/gorilla/mux v1.7.3
	github.com/grpc-ecosystem/go-grpc-middleware v1.2.2
	github.com/hashicorp/go-uuid v1.0.2 // indirect
	github.com/hashicorp/go-version v1.2.1
	github.com/hyperledger/fabric-amcl v0.0.0-20200424173818-327c9e2cf77a
	github.com/hyperledger/fabric-lib-go v1.0.0
	github.com/inconshreveable/mousetrap v1.0.0
	github.com/jcmturner/gofork v1.0.0 // indirect
	github.com/klauspost/compress v1.11.7 // indirect
	github.com/konsorten/go-windows-terminal-sequences v1.0.3
	github.com/kr/logfmt v0.0.0-20140226030751-b84e30acd515
	github.com/kr/pretty v0.2.1
	github.com/kr/text v0.2.0
	github.com/magiconair/properties v1.8.4
	github.com/mattn/go-runewidth v0.0.10
	github.com/matttproud/golang_protobuf_extensions v1.0.1
	github.com/miekg/pkcs11 v1.0.3
	github.com/mitchellh/mapstructure v1.4.1
	github.com/onsi/ginkgo v1.14.0
	github.com/onsi/gomega v1.10.4
	github.com/op/go-logging v0.0.0-20160315200505-970db520ece7
	github.com/opencontainers/go-digest v1.0.0
	github.com/opencontainers/image-spec v1.0.1
	github.com/opencontainers/runc v0.1.1
	github.com/pierrec/lz4 v2.6.0+incompatible
	github.com/pkg/errors v0.9.1
	github.com/pmezard/go-difflib v1.0.0
	github.com/prometheus/client_golang v1.7.1
	github.com/prometheus/client_model v0.2.0
	github.com/prometheus/common v0.15.0
	github.com/prometheus/procfs v0.3.0
	github.com/rcrowley/go-metrics v0.0.0-20201227073835-cf1acfcdf475
	github.com/rivo/uniseg v0.2.0 // indirect
	github.com/sirupsen/logrus v1.7.0
	github.com/spf13/cast v1.3.1
	github.com/spf13/cobra v0.0.7
	github.com/spf13/jwalterweatherman v1.1.0
	github.com/spf13/pflag v1.0.3
	github.com/spf13/viper v1.4.0
	github.com/stretchr/objx v0.3.0
	github.com/stretchr/testify v1.7.0
	github.com/sykesm/zap-logfmt v0.0.4
	github.com/syndtr/goleveldb v1.0.1-0.20200815110645-5c35d600f0ca
	github.com/tedsuo/ifrit v0.0.0-20191009134036-9a97d0632f00
	github.com/willf/bitset v1.1.11
	github.com/zhigui-projects/gm-crypto v0.0.0-20200719051209-13ea42f5b80c
	github.com/zhigui-projects/gm-go v0.0.0-20200510034956-8e4ef670d055
	github.com/zhigui-projects/gm-plugins v0.0.0-20200721031044-dc235c6ce0d5
	go.etcd.io/etcd v0.0.0-20191023171146-3cf2f69b5738
	go.uber.org/atomic v1.7.0
	go.uber.org/multierr v1.6.0
	go.uber.org/zap v1.16.0
	golang.org/x/crypto v0.0.0-20201221181555-eec23a3978ad
	golang.org/x/lint v0.0.0-20190930215403-16217165b5de
	golang.org/x/net v0.0.0-20210119194325-5f4716e94777
	golang.org/x/sync v0.0.0-20201207232520-09787c993a3a
	golang.org/x/sys v0.0.0-20210124154548-22da62e12c0c
	golang.org/x/text v0.3.5
	golang.org/x/tools v0.0.0-20210106214847-113979e3529a
	golang.org/x/xerrors v0.0.0-20200804184101-5ec99f83aff1 // indirect
	google.golang.org/genproto v0.0.0-20210125195502-f46fe6c6624a
	google.golang.org/grpc v1.29.1
	google.golang.org/protobuf v1.25.0
	gopkg.in/alecthomas/kingpin.v2 v2.2.6
	gopkg.in/cheggaaa/pb.v1 v1.0.25
	gopkg.in/jcmturner/aescts.v1 v1.0.1 // indirect
	gopkg.in/jcmturner/dnsutils.v1 v1.0.1 // indirect
	gopkg.in/jcmturner/gokrb5.v7 v7.5.0 // indirect
	gopkg.in/jcmturner/rpc.v1 v1.1.0 // indirect
	gopkg.in/yaml.v2 v2.4.0
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b // indirect
)
