package blockpool

import (
	"fmt"
	"io"

	"github.com/go-errors/errors"
	"github.com/itchio/wharf/pwr"
	"github.com/itchio/wharf/sync"
	"github.com/itchio/wharf/tlc"
)

// A BlockPool implements a pool that maps reads, seeks, and writes to blocks
type BlockPool struct {
	Container *tlc.Container
	BlockSize int64

	Upstream   Source
	Downstream Sink
	Consumer   *pwr.StateConsumer

	reader *BlockPoolReader
}

var _ sync.Pool = (*BlockPool)(nil)
var _ sync.WritablePool = (*BlockPool)(nil)

func (np *BlockPool) GetReader(fileIndex int64) (io.Reader, error) {
	return np.GetReadSeeker(fileIndex)
}

func (np *BlockPool) GetReadSeeker(fileIndex int64) (io.ReadSeeker, error) {
	if np.Upstream == nil {
		return nil, errors.Wrap(fmt.Errorf("BlockPool: no upstream"), 1)
	}

	if np.reader != nil {
		if np.reader.FileIndex == fileIndex {
			return np.reader, nil
		}

		err := np.reader.Close()
		if err != nil {
			return nil, err
		}
		np.reader = nil
	}

	np.reader = &BlockPoolReader{
		Pool:      np,
		FileIndex: fileIndex,

		offset: 0,
		size:   np.Container.Files[fileIndex].Size,

		blockIndex: -1,
		blockBuf:   make([]byte, np.BlockSize),
	}
	return np.reader, nil
}

func (np *BlockPool) GetWriter(fileIndex int64) (io.WriteCloser, error) {
	if np.Downstream == nil {
		return nil, errors.Wrap(fmt.Errorf("BlockPool: no downstream"), 1)
	}

	npw := &BlockPoolWriter{
		Pool:      np,
		FileIndex: fileIndex,

		offset:   0,
		size:     np.Container.Files[fileIndex].Size,
		blockBuf: make([]byte, np.BlockSize),
	}
	return npw, nil
}

func (np *BlockPool) Close() error {
	if np.reader != nil {
		err := np.reader.Close()
		if err != nil {
			return err
		}
		np.reader = nil
	}

	return nil
}
