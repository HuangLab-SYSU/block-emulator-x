package block

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"go.etcd.io/bbolt"

	"github.com/HuangLab-SYSU/block-emulator-x/config"
)

const (
	bucketBlock       = "block"
	bucketBlockHeader = "block_header"
	bucketNewestBlock = "newest_block"

	newestBlockKey = "newest_block"

	boltDBFilePathFmt = "/shard_%d_node_%d/block.db"
)

// BoltStore implements block.Store.
type BoltStore struct {
	db *bbolt.DB
}

func NewBoltStore(cfg config.BoltCfg, lp config.LocalParams) (*BoltStore, error) {
	fp := filepath.Join(cfg.FilePathDir, fmt.Sprintf(boltDBFilePathFmt, lp.ShardID, lp.NodeID))
	if err := os.MkdirAll(filepath.Dir(fp), os.ModePerm); err != nil {
		return nil, fmt.Errorf("create bolt db dir: %w", err)
	}

	db, err := bbolt.Open(fp, 0o600, nil)
	if err != nil {
		return nil, fmt.Errorf("open bolt db err: %w", err)
	}

	err = db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(bucketBlock))
		if err != nil {
			return fmt.Errorf("create bucket %s: %w", bucketBlock, err)
		}

		_, err = tx.CreateBucketIfNotExists([]byte(bucketBlockHeader))
		if err != nil {
			return fmt.Errorf("create bucket %s: %w", bucketBlockHeader, err)
		}

		_, err = tx.CreateBucketIfNotExists([]byte(bucketNewestBlock))
		if err != nil {
			return fmt.Errorf("create bucket %s: %w", bucketNewestBlock, err)
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
		bucket := tx.Bucket([]byte(bucketNewestBlock))
		if bucket == nil {
			return fmt.Errorf("fetch bucketNewestBlock failed, bucket is nil")
		}

		err := bucket.Put([]byte(newestBlockKey), newBlockHash)
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
		bucket := tx.Bucket([]byte(bucketNewestBlock))
		if bucket == nil {
			return fmt.Errorf("fetch bucketNewestBlock failed, bucket is nil")
		}

		result = bucket.Get([]byte(newestBlockKey))
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
		headerBucket := tx.Bucket([]byte(bucketBlockHeader))
		if headerBucket == nil {
			return fmt.Errorf("fetch bucketBlockHeader failed, headerBucket is nil")
		}

		err := headerBucket.Put(blockHash, encodedBlockHeader)
		if err != nil {
			return fmt.Errorf("put blockHeader err: %w", err)
		}

		// update the newest block hash
		newestBlockBucket := tx.Bucket([]byte(bucketNewestBlock))
		if newestBlockBucket == nil {
			return fmt.Errorf("fetch bucketNewestBlock failed, newestBlockBucket is nil")
		}

		err = newestBlockBucket.Put([]byte(newestBlockKey), blockHash)
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
		bucket := tx.Bucket([]byte(bucketBlockHeader))
		if bucket == nil {
			return fmt.Errorf("fetch bucketBlockHeader failed, bucket is nil")
		}

		result = bucket.Get(blockHash)
		if result == nil {
			return fmt.Errorf("get blockHash in the bucketBlockHeader failed")
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
		blockBucket := tx.Bucket([]byte(bucketBlock))
		if blockBucket == nil {
			return fmt.Errorf("fetch bucketBlock failed, bucket is nil")
		}

		err := blockBucket.Put(blockHash, encodedBlock)
		if err != nil {
			return fmt.Errorf("put block err: %w", err)
		}

		// add block header
		headerBucket := tx.Bucket([]byte(bucketBlockHeader))
		if headerBucket == nil {
			return fmt.Errorf("fetch bucketBlockHeader failed, headerBucket is nil")
		}

		err = headerBucket.Put(blockHash, encodedBlockHeader)
		if err != nil {
			return fmt.Errorf("put blockHeader err: %w", err)
		}

		// update the newest block hash
		newestBlockBucket := tx.Bucket([]byte(bucketNewestBlock))
		if newestBlockBucket == nil {
			return fmt.Errorf("fetch bucketNewestBlock failed, newestBlockBucket is nil")
		}

		err = newestBlockBucket.Put([]byte(newestBlockKey), blockHash)
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
		bucket := tx.Bucket([]byte(bucketBlock))
		if bucket == nil {
			return fmt.Errorf("fetch bucketBlock failed, bucket is nil")
		}

		result = bucket.Get(hash)
		if result == nil {
			return fmt.Errorf("get block by hash in bucketBlock failed")
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (b *BoltStore) Close() error {
	return b.db.Close()
}
