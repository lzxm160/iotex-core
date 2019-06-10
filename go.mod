module github.com/iotexproject/iotex-core

go 1.12

require (
	cloud.google.com/go v0.40.0 // indirect
	github.com/aristanetworks/goarista v0.0.0-20190607111240-52c2a7864a08 // indirect
	github.com/btcsuite/btcd v0.0.0-20190605094302-a0d1e3e36d50 // indirect
	github.com/cenkalti/backoff v2.1.1+incompatible
	github.com/coreos/bbolt v1.3.3 // indirect
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
	github.com/iotexproject/iotex-election v0.1.10
	github.com/iotexproject/iotex-proto v0.2.1-0.20190528210926-c48a31f9d016
	github.com/libp2p/go-libp2p-peerstore v0.1.0
	github.com/libp2p/go-msgio v0.0.3 // indirect
	github.com/mattn/go-sqlite3 v1.10.0
	github.com/minio/blake2b-simd v0.0.0-20160723061019-3f5f724cb5b1
	github.com/multiformats/go-multiaddr v0.0.4
	github.com/pkg/errors v0.8.1
	github.com/prometheus/client_golang v0.9.4
	github.com/rs/zerolog v1.14.3
	github.com/spf13/cobra v0.0.5
	github.com/stretchr/testify v1.3.0
	go.etcd.io/bbolt v1.3.3
	go.uber.org/automaxprocs v1.2.0
	go.uber.org/config v1.3.1
	go.uber.org/zap v1.10.0
	golang.org/x/crypto v0.0.0-20190605123033-f99c8df09eb5
	golang.org/x/mobile v0.0.0-20190607214518-6fa95d984e88 // indirect
	golang.org/x/net v0.0.0-20190607181551-461777fb6f67
	golang.org/x/sync v0.0.0-20190423024810-112230192c58
	golang.org/x/sys v0.0.0-20190609082536-301114b31cce // indirect
	golang.org/x/tools v0.0.0-20190608022120-eacb66d2a7c3 // indirect
	google.golang.org/appengine v1.6.1 // indirect
	google.golang.org/genproto v0.0.0-20190605220351-eb0b1bdb6ae6 // indirect
	google.golang.org/grpc v1.21.1
	gopkg.in/yaml.v2 v2.2.2
	honnef.co/go/tools v0.0.0-20190607181801-497c8f037f5a // indirect
)

replace github.com/ethereum/go-ethereum => github.com/iotexproject/go-ethereum v1.7.4-0.20190604221806-8ab2d21b162f
