package broker

import (
	"bufio"
	"fmt"
	"log/slog"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/HuangLab-SYSU/block-emulator-x/config"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/core/account"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/core/transaction"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/utils"
)

type rawTxHash string

const (
	stageWaitConfirm1 = 1
	stageWaitConfirm2 = 2
)

type txInfoStage struct {
	rawTx        transaction.Transaction
	confirmStage int
}

type Manager struct {
	broker2Nonce      map[account.Address]uint64
	unconfirmedTxInfo map[rawTxHash]*txInfoStage // unconfirmed transactions information

	readyBroker1TxHashes, readyBroker2TxHashes []rawTxHash // the hash of transactions those are ready to be created

	cfg config.BrokerModuleCfg

	brokers []account.Address
}

func NewBrokerManager(cfg config.BrokerModuleCfg) (*Manager, error) {
	bs, err := readBrokersFromFile(cfg)
	if err != nil {
		return nil, fmt.Errorf("read brokers: %w", err)
	}

	bList := make([]account.Address, 0, len(bs))
	for b := range bs {
		bList = append(bList, b)
	}

	return &Manager{
		broker2Nonce:         bs,
		unconfirmedTxInfo:    make(map[rawTxHash]*txInfoStage),
		readyBroker1TxHashes: make([]rawTxHash, 0),
		readyBroker2TxHashes: make([]rawTxHash, 0),
		brokers:              bList,
		cfg:                  cfg,
	}, nil
}

func (s *Manager) IsBroker(addr account.Address) bool {
	_, ok := s.broker2Nonce[addr]
	return ok
}

func (s *Manager) CreateRawTxsRandomBroker(txs []transaction.Transaction) ([]transaction.Transaction, error) {
	rawTxs := make([]transaction.Transaction, len(txs))
	for i, tx := range txs {
		broker := s.brokers[rand.Intn(len(s.brokers))]

		rawTx, err := s.CreateRawTx(tx, broker)
		if err != nil {
			return nil, fmt.Errorf("CreateRawTx failed: %w", err)
		}

		rawTxs[i] = *rawTx
	}

	return rawTxs, nil
}

// CreateRawTx creates a raw tx with the given tx.
func (s *Manager) CreateRawTx(
	tx transaction.Transaction,
	brokerAddr account.Address,
) (*transaction.Transaction, error) {
	th, err := tx.Hash()
	if err != nil {
		return nil, fmt.Errorf("calc hash: %w", err)
	}

	if !s.IsBroker(brokerAddr) {
		return nil, fmt.Errorf("%x is not a broker address", brokerAddr)
	}

	if _, ok := s.unconfirmedTxInfo[rawTxHash(th)]; ok {
		return nil, fmt.Errorf("tx hash %x already exists in the unconfirmed tx set", th)
	}

	s.broker2Nonce[brokerAddr]++
	rawTx := tx
	rawTx.BrokerTxOpt = transaction.BrokerTxOpt{
		BrokerStage:          transaction.RawTxBrokerStage,
		Broker:               brokerAddr,
		BOriginalHash:        th,
		OriginalTxCreateTime: tx.CreateTime,
		NonceBroker:          s.broker2Nonce[brokerAddr],
	}

	// add this hash to the pool, for the further operations
	s.unconfirmedTxInfo[rawTxHash(th)] = &txInfoStage{
		rawTx:        rawTx,
		confirmStage: stageWaitConfirm1,
	}
	s.readyBroker1TxHashes = append(s.readyBroker1TxHashes, rawTxHash(th))

	return &rawTx, nil
}

// CreateBrokerTxs create broker1 and broker2 txs according to the readyBrokerTxHashes.
func (s *Manager) CreateBrokerTxs() ([]transaction.Transaction, []transaction.Transaction) {
	b1Txs := make([]transaction.Transaction, 0, len(s.readyBroker1TxHashes))

	b2Txs := make([]transaction.Transaction, 0, len(s.readyBroker2TxHashes))

	for _, tx := range s.readyBroker1TxHashes {
		b1Tx, err := s.createBroker1Tx(tx)
		if err != nil {
			slog.Error("create broker1 tx failed", "err", err)
			continue
		}

		b1Txs = append(b1Txs, *b1Tx)
	}

	// Clear readyBroker1TxHashes
	s.readyBroker1TxHashes = make([]rawTxHash, 0)

	for _, tx := range s.readyBroker2TxHashes {
		b2Tx, err := s.createBroker2Tx(tx)
		if err != nil {
			slog.Error("create broker2 tx failed", "err", err)
			continue
		}

		b2Txs = append(b2Txs, *b2Tx)
	}

	// Clear readyBroker2TxHashes
	s.readyBroker2TxHashes = make([]rawTxHash, 0)

	return b1Txs, b2Txs
}

// ConfirmBrokerTx confirms the broker tx according to Manager local data
func (s *Manager) ConfirmBrokerTx(tx transaction.Transaction) error {
	if !s.IsBroker(tx.Broker) {
		return fmt.Errorf("%x is not a broker address", tx.Broker)
	}

	infoStages, ok := s.unconfirmedTxInfo[rawTxHash(tx.BOriginalHash)]
	if !ok {
		return fmt.Errorf("not a recorded tx")
	}

	switch infoStages.confirmStage {
	case stageWaitConfirm1:
		if tx.BrokerStage != transaction.Sigma1BrokerStage {
			return fmt.Errorf("invalid confirm stage = %d, expect wait-confirm-1", tx.BrokerStage)
		}

		infoStages.confirmStage = stageWaitConfirm2
		// add this raw tx hash to the pending list
		s.readyBroker2TxHashes = append(s.readyBroker2TxHashes, rawTxHash(tx.BOriginalHash))
	case stageWaitConfirm2:
		if tx.BrokerStage != transaction.Sigma2BrokerStage {
			return fmt.Errorf("invalid confirm stage = %d, expect wait-confirm-2", tx.BrokerStage)
		}

		delete(s.unconfirmedTxInfo, rawTxHash(tx.BOriginalHash))
	default:
		return fmt.Errorf("invalid confirm stage = %d", infoStages.confirmStage)
	}

	return nil
}

// createBroker1Tx creates broker1 tx with the given raw tx.
func (s *Manager) createBroker1Tx(txHash rawTxHash) (*transaction.Transaction, error) {
	infoStage, ok := s.unconfirmedTxInfo[txHash]
	if !ok {
		return nil, fmt.Errorf("not a recorded raw tx")
	} else if infoStage.confirmStage != stageWaitConfirm1 {
		return nil, fmt.Errorf("invalid confirm stage = %d, expect wait-confirm-1", infoStage.confirmStage)
	}

	b1Tx := infoStage.rawTx
	b1Tx.BrokerStage = transaction.Sigma1BrokerStage
	b1Tx.CreateTime = time.Now()

	return &b1Tx, nil
}

// createBroker2Tx creates broker2 tx with the given broker1 tx.
func (s *Manager) createBroker2Tx(txHash rawTxHash) (*transaction.Transaction, error) {
	infoStage, ok := s.unconfirmedTxInfo[txHash]
	if !ok {
		return nil, fmt.Errorf("not a recorded raw tx")
	} else if infoStage.confirmStage != stageWaitConfirm2 {
		return nil, fmt.Errorf("invalid confirm stage = %d, expect wait-confirm-2", infoStage.confirmStage)
	}

	b2Tx := infoStage.rawTx
	b2Tx.BrokerStage = transaction.Sigma2BrokerStage
	b2Tx.CreateTime = time.Now()

	return &b2Tx, nil
}

func readBrokersFromFile(cfg config.BrokerModuleCfg) (map[account.Address]uint64, error) {
	brokerSet := make(map[account.Address]uint64)

	f, err := os.Open(cfg.BrokerFilePath)
	if err != nil {
		return nil, fmt.Errorf("open broker file: %w", err)
	}

	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	scanner.Split(bufio.ScanLines)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// ignore empty lines
		if line == "" {
			continue
		}

		var addr account.Address

		if addr, err = utils.Hex2Addr(line); err != nil {
			return nil, fmt.Errorf("invalid broker address %q: %w", line, err)
		}

		brokerSet[addr] = 0

		if int64(len(brokerSet)) >= cfg.BrokerNum {
			break
		}
	}

	if err = scanner.Err(); err != nil {
		return nil, fmt.Errorf("read broker file: %w", err)
	}

	return brokerSet, nil
}
