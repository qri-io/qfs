module github.com/qri-io/qfs

go 1.12

replace github.com/qri-io/value => /Users/b5/qri/value

replace (
	github.com/go-critic/go-critic v0.0.0-20181204210945-c3db6069acc5 => github.com/go-critic/go-critic v0.0.0-20190422201921-c3db6069acc5
	github.com/go-critic/go-critic v0.0.0-20181204210945-ee9bf5809ead => github.com/go-critic/go-critic v0.0.0-20190210220443-ee9bf5809ead
	github.com/golangci/errcheck v0.0.0-20181003203344-ef45e06d44b6 => github.com/golangci/errcheck v0.0.0-20181223084120-ef45e06d44b6
	github.com/golangci/go-tools v0.0.0-20180109140146-af6baa5dc196 => github.com/golangci/go-tools v0.0.0-20190318060251-af6baa5dc196
	github.com/golangci/gofmt v0.0.0-20181105071733-0b8337e80d98 => github.com/golangci/gofmt v0.0.0-20181222123516-0b8337e80d98
	github.com/golangci/gosec v0.0.0-20180901114220-66fb7fc33547 => github.com/golangci/gosec v0.0.0-20190211064107-66fb7fc33547
	github.com/golangci/lint-1 v0.0.0-20180610141402-ee948d087217 => github.com/golangci/lint-1 v0.0.0-20190420132249-ee948d087217
	mvdan.cc/unparam v0.0.0-20190124213536-fbb59629db34 => mvdan.cc/unparam v0.0.0-20190209190245-fbb59629db34
)

require (
	github.com/google/go-cmp v0.3.1
	github.com/ipfs/go-block-format v0.0.2
	github.com/ipfs/go-blockservice v0.1.2
	github.com/ipfs/go-cid v0.0.3
	github.com/ipfs/go-datastore v0.1.0
	github.com/ipfs/go-ipfs v0.4.22-0.20191023033800-4a102207a36c
	github.com/ipfs/go-ipfs-blockstore v0.1.0
	github.com/ipfs/go-ipfs-chunker v0.0.3
	github.com/ipfs/go-ipfs-config v0.0.11
	github.com/ipfs/go-ipfs-exchange-offline v0.0.1
	github.com/ipfs/go-ipfs-http-client v0.0.5
	github.com/ipfs/go-ipfs-posinfo v0.0.1
	github.com/ipfs/go-ipfs-util v0.0.1
	github.com/ipfs/go-ipld-cbor v0.0.3
	github.com/ipfs/go-ipld-format v0.0.2
	github.com/ipfs/go-log v0.0.1
	github.com/ipfs/go-merkledag v0.2.3
	github.com/ipfs/go-mfs v0.1.1
	github.com/ipfs/go-path v0.0.7
	github.com/ipfs/go-unixfs v0.2.1
	github.com/ipfs/interface-go-ipfs-core v0.2.3
	github.com/libp2p/go-libp2p v0.4.0
	github.com/libp2p/go-libp2p-core v0.2.3
	github.com/libp2p/go-libp2p-peerstore v0.1.3
	github.com/lucas-clemente/quic-go-certificates v0.0.0-20160823095156-d2f86524cced // indirect
	github.com/mr-tron/base58 v1.1.2
	github.com/multiformats/go-multibase v0.0.1
	github.com/multiformats/go-multihash v0.0.8
	github.com/qri-io/value v0.0.0-00010101000000-000000000000
	github.com/ugorji/go/codec v0.0.0-20181204163529-d75b2dcb6bc8
	golang.org/x/sys v0.0.0-20190926180325-855e68c8590b
)
