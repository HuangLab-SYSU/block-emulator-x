package block

import (
	"context"
	"fmt"

	"github.com/HuangLab-SYSU/block-emulator/config"
	"go.etcd.io/bbolt"
)

const (
	BucketBlock       = "block"
	BucketBlockHeader = "block_header"
	BucketNewestBlock = "newest_block"

	NewestBlockKey = "newest_block"
)

// BoltStore implements block.Store.
type BoltStore struct {
	db *bbolt.DB
}

func NewBoltStore(cfg *config.BoltCfg) (*BoltStore, error) {
	db, err := bbolt.Open(cfg.FilePath, 0600, nil)
	if err != nil {
		return nil, fmt.Errorf("open bolt db err: %w", err)
	}

	err = db.Update(func(tx *bbolt.Tx) error {
		var err error
		_, err = tx.CreateBucketIfNotExists([]byte(BucketBlock))
		if err != nil {
			return err
		}
		_, err = tx.CreateBucketIfNotExists([]byte(BucketBlockHeader))
		if err != nil {
			return err
		}
		_, err = tx.CreateBucketIfNotExists([]byte(BucketNewestBlock))
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("init db bucket err: %w", err)
	}

	return &BoltStore{
		db: db,
	}, nil
}

func (b *BoltStore) UpdateNewestBlockHash(_ context.Context, newBlockHash []byte) error {
	err := b.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(BucketNewestBlock))
		if bucket == nil {
			return fmt.Errorf("fetch BucketNewestBlock failed, bucket is nil")
		}
		err := bucket.Put([]byte(NewestBlockKey), newBlockHash)
		if err != nil {
			return fmt.Errorf("put newest blockHash err: %w", err)
		}
		return nil
	})

	if err != nil {
		return err
	}
	return nil
}

func (b *BoltStore) GetNewestBlockHash(_ context.Context) ([]byte, error) {
	var result []byte
	err := b.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(BucketNewestBlock))
		if bucket == nil {
			return fmt.Errorf("fetch BucketNewestBlock failed, bucket is nil")
		}
		result = bucket.Get([]byte(NewestBlockKey))
		if result == nil {
			return fmt.Errorf("get newest blockHash failed")
		}
		return nil
	})

	if err != nil {
		return nil, err
	}
	return result, nil
}

func (b *BoltStore) AddBlockHeader(_ context.Context, blockHash, encodedBlockHeader []byte) error {
	err := b.db.Update(func(tx *bbolt.Tx) error {
		// add block header first
		headerBucket := tx.Bucket([]byte(BucketBlockHeader))
		if headerBucket == nil {
			return fmt.Errorf("fetch BucketBlockHeader failed, headerBucket is nil")
		}
		err := headerBucket.Put(blockHash, encodedBlockHeader)
		if err != nil {
			return fmt.Errorf("put blockHeader err: %w", err)
		}

		// update the newest block hash
		newestBlockBucket := tx.Bucket([]byte(BucketNewestBlock))
		if newestBlockBucket == nil {
			return fmt.Errorf("fetch BucketNewestBlock failed, newestBlockBucket is nil")
		}
		err = newestBlockBucket.Put([]byte(NewestBlockKey), blockHash)
		if err != nil {
			return fmt.Errorf("update newest blockHash err: %w", err)
		}

		return nil
	})

	if err != nil {
		return err
	}
	return nil
}

func (b *BoltStore) GetBlockHeaderByHash(_ context.Context, blockHash []byte) ([]byte, error) {
	var result []byte
	err := b.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(BucketBlockHeader))
		if bucket == nil {
			return fmt.Errorf("fetch BucketBlockHeader failed, bucket is nil")
		}
		result = bucket.Get(blockHash)
		if result == nil {
			return fmt.Errorf("get blockHash in the BucketBlockHeader failed")
		}
		return nil
	})

	if err != nil {
		return nil, err
	}
	return result, nil
}

func (b *BoltStore) AddBlock(_ context.Context, blockHash, encodedBlock, encodedBlockHeader []byte) error {
	err := b.db.Update(func(tx *bbolt.Tx) error {
		// add block first
		blockBucket := tx.Bucket([]byte(BucketBlock))
		if blockBucket == nil {
			return fmt.Errorf("fetch BucketBlock failed, bucket is nil")
		}
		err := blockBucket.Put(blockHash, encodedBlock)
		if err != nil {
			return fmt.Errorf("put block err: %w", err)
		}

		// add block header
		headerBucket := tx.Bucket([]byte(BucketBlockHeader))
		if headerBucket == nil {
			return fmt.Errorf("fetch BucketBlockHeader failed, headerBucket is nil")
		}
		err = headerBucket.Put(blockHash, encodedBlockHeader)
		if err != nil {
			return fmt.Errorf("put blockHeader err: %w", err)
		}

		// update the newest block hash
		newestBlockBucket := tx.Bucket([]byte(BucketNewestBlock))
		if newestBlockBucket == nil {
			return fmt.Errorf("fetch BucketNewestBlock failed, newestBlockBucket is nil")
		}
		err = newestBlockBucket.Put([]byte(NewestBlockKey), blockHash)
		if err != nil {
			return fmt.Errorf("update newest blockHash err: %w", err)
		}
		return nil
	})

	if err != nil {
		return err
	}
	return nil
}

func (b *BoltStore) GetBlockByHash(_ context.Context, hash []byte) ([]byte, error) {
	var result []byte
	err := b.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(BucketBlock))
		if bucket == nil {
			return fmt.Errorf("fetch BucketBlock failed, bucket is nil")
		}
		result = bucket.Get(hash)
		if result == nil {
			return fmt.Errorf("get block by hash in BucketBlock failed")
		}
		return nil
	})

	if err != nil {
		return nil, err
	}
	return result, nil
}
