package filedao

import (
	"github.com/golang/protobuf/proto"

	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/blockchain/block"
)

type (
	stagingBuffer struct {
		size   uint64
		buffer []*block.Store
	}
)

func newStagingBuffer(size uint64) *stagingBuffer {
	return &stagingBuffer{
		size:   size,
		buffer: make([]*block.Store, size),
	}
}

func (s *stagingBuffer) Get(pos uint64) (*block.Store, error) {
	if pos >= s.size {
		return nil, ErrNotSupported
	}
	return s.buffer[pos], nil
}

func (s *stagingBuffer) Put(pos uint64, blkBytes []byte) (bool, error) {
	if pos >= s.size {
		return false, ErrNotSupported
	}
	blk := &block.Store{}
	if err := blk.Deserialize(blkBytes); err != nil {
		return false, err
	}
	s.buffer[pos] = blk
	return pos == s.size-1, nil
}

func (s *stagingBuffer) Serialize() ([]byte, error) {
	blkStores := []*iotextypes.BlockStore{}
	for _, v := range s.buffer {
		blkStores = append(blkStores, v.ToProto())
	}
	allBlks := &iotextypes.BlockStores{
		BlockStores: blkStores,
	}
	return proto.Marshal(allBlks)
}
