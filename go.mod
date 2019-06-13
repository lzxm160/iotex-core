module github.com/iotexproject/iotex-core

go 1.12

require (
	github.com/cenkalti/backoff v2.1.1+incompatible
	github.com/dgraph-io/badger v2.0.0-rc.2+incompatible
	github.com/ethereum/go-ethereum v1.8.27
	github.com/facebookgo/clock v0.0.0-20150410010913-600d898af40a
	github.com/go-sql-driver/mysql v1.4.1
	github.com/gogo/protobuf v1.2.1
	github.com/golang/groupcache v0.0.0-20190129154638-5b532d6fd5ef
	github.com/golang/mock v1.3.1
	github.com/golang/protobuf v1.3.1
	github.com/grpc-ecosystem/go-grpc-prometheus v1.2.0
	github.com/iotexproject/go-fsm v1.0.0
	github.com/iotexproject/go-p2p v0.2.10
	github.com/iotexproject/go-pkgs v0.1.1-0.20190513193226-f065b9342b78
	github.com/iotexproject/iotex-address v0.2.0
	github.com/iotexproject/iotex-election v0.1.14
	github.com/iotexproject/iotex-proto v0.2.1-0.20190528210926-c48a31f9d016
	github.com/libp2p/go-libp2p-peerstore v0.0.5
	github.com/mattn/go-sqlite3 v1.10.0
	github.com/minio/blake2b-simd v0.0.0-20160723061019-3f5f724cb5b1
	github.com/multiformats/go-multiaddr v0.0.2
	github.com/pkg/errors v0.8.1
	github.com/prometheus/client_golang v0.9.3
	github.com/rs/zerolog v1.14.3
	github.com/spf13/cobra v0.0.4
	github.com/stretchr/testify v1.3.0
	go.etcd.io/bbolt v1.3.2
	go.uber.org/automaxprocs v1.2.0
	go.uber.org/config v1.3.1
	go.uber.org/zap v1.10.0
	golang.org/x/crypto v0.0.0-20190530122614-20be4c3c3ed5
	golang.org/x/net v0.0.0-20190603091049-60506f45cf65
	golang.org/x/sync v0.0.0-20190423024810-112230192c58
	google.golang.org/grpc v1.21.0
	gopkg.in/yaml.v2 v2.2.2
)

replace github.com/ethereum/go-ethereum => github.com/iotexproject/go-ethereum v1.7.4-0.20190216004546-2bbee71fbe61

replace github.com/iotexproject/iotex-election => github.com/lzxm160/iotex-election v0.3.20
