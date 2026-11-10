package committee

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
)

// Externally computed partitions enter BE-X through these files: an incremental location table
// computed off-chain. The loaded map is broadcast as a native message.CLPARepartitionStartMsg, and
// the account migration and epoch sync that follow are the native path, indistinguishable from a
// partition CLPA produced on-chain.
//
// File layout (little endian):
//
//	"MTPART1\0" | u32 K | u32 n | n × (20-byte address | u32 destination shard)
//
// The n records are the accounts whose location changes in this round, with the same meaning as
// the map partition.CLPAPartition returns. A record whose destination shard equals the account's
// current location must not appear; the migration side rejects it with destShardID == srcShardID.
const (
	partitionFileMagic  = "MTPART1\x00"
	partitionFileHeader = 16 // magic(8) + K(4) + n(4)
	partitionEntrySize  = 24 // address(20) + shard(4)
)

// partitionFilePath returns the file that holds the location table of the given epoch.
func partitionFilePath(dir string, epoch int) string {
	return filepath.Join(dir, fmt.Sprintf("epoch_%04d.bin", epoch))
}

// parseEpochPartitionHeader validates the header of one epoch partition file and returns the
// record count.
func parseEpochPartitionHeader(path string, raw []byte, shardNum int) (int, error) {
	if len(raw) < partitionFileHeader || string(raw[:8]) != partitionFileMagic {
		return 0, fmt.Errorf("%s is not an epoch-partition file (bad magic or truncated header)", path)
	}

	k := int(binary.LittleEndian.Uint32(raw[8:12]))
	n := int(binary.LittleEndian.Uint32(raw[12:16]))

	if k != shardNum {
		return 0, fmt.Errorf("%s was built for K=%d but shard_num=%d", path, k, shardNum)
	}

	if len(raw) != partitionFileHeader+n*partitionEntrySize {
		return 0, fmt.Errorf("%s is truncated: %d bytes, want %d for n=%d",
			path, len(raw), partitionFileHeader+n*partitionEntrySize, n)
	}

	return n, nil
}

// readEpochPartitionHeader reads the file and validates only its header.
func readEpochPartitionHeader(path string, shardNum int) (int, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0, fmt.Errorf("read %s: %w", path, err)
	}

	return parseEpochPartitionHeader(path, raw, shardNum)
}

// LoadEpochPartition reads the incremental location table of one epoch. Epochs start at 1, the
// same numbering message.CLPARepartitionStartMsg.Epoch uses.
func LoadEpochPartition(dir string, epoch, shardNum int) (map[[20]byte]int, error) {
	path := partitionFilePath(dir, epoch)

	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	n, err := parseEpochPartitionHeader(path, raw, shardNum)
	if err != nil {
		return nil, err
	}

	out := make(map[[20]byte]int, n)

	for i := 0; i < n; i++ {
		off := partitionFileHeader + i*partitionEntrySize

		var addr [20]byte
		copy(addr[:], raw[off:off+20])

		dest := int(binary.LittleEndian.Uint32(raw[off+20 : off+24]))
		if dest >= shardNum {
			return nil, fmt.Errorf("%s: entry %d has shard=%d out of range [0,%d)",
				path, i, dest, shardNum)
		}

		out[addr] = dest
	}

	return out, nil
}

// CheckEpochPartitions validates every partition file this run may need and returns how many
// epochs are covered. A missing file has to fail here: the injection loop only logs repartition
// errors and keeps injecting, so a whole run would finish without any repartition while the
// result files still looked normal.
func CheckEpochPartitions(dir string, maxEpoch, shardNum int) (int, error) {
	if dir == "" {
		return 0, nil
	}

	for e := 1; e <= maxEpoch; e++ {
		if _, err := readEpochPartitionHeader(partitionFilePath(dir, e), shardNum); err != nil {
			return 0, fmt.Errorf("epoch %d of %d: %w", e, maxEpoch, err)
		}
	}

	return maxEpoch, nil
}
