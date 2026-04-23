package main

import (
	"crypto/rand"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

var csvHeader = []string{
	"blockNumber", "timestamp", "transactionHash", "from", "to", "toCreate",
	"fromIsContract", "toIsContract", "value", "gasLimit", "gasPrice", "gasUsed",
	"callingFunction", "isError", "eip2718type", "baseFeePerGas", "maxFeePerGas", "maxPriorityFeePerGas",
}

type Config struct {
	Seed      int64                   `json:"seed"`
	Output    OutputConfig            `json:"output"`
	Accounts  map[string]string       `json:"accounts"`
	Contracts map[string]ContractSpec `json:"contracts"`
	Scenarios []Scenario              `json:"scenarios"`
}

type OutputConfig struct {
	CSVPath               string `json:"csv_path"`
	StartBlockNumber      int64  `json:"start_block_number"`
	StartTimestamp        int64  `json:"start_timestamp"`
	BlockStep             int64  `json:"block_step"`
	TimestampStep         int64  `json:"timestamp_step"`
	DefaultGasPrice       string `json:"default_gas_price"`
	DefaultGasUsed        string `json:"default_gas_used"`
	DefaultIsError        string `json:"default_is_error"`
	DefaultEIP2718Type    string `json:"default_eip2718type"`
	DefaultBaseFeePerGas  string `json:"default_base_fee_per_gas"`
	DefaultMaxFeePerGas   string `json:"default_max_fee_per_gas"`
	DefaultMaxPriorityFee string `json:"default_max_priority_fee_per_gas"`
}

type ContractSpec struct {
	ABIPath  string `json:"abi_path"`
	Bytecode string `json:"bytecode"`
}

type Scenario struct {
	Name  string `json:"name"`
	Steps []Step `json:"steps"`
}

type Step struct {
	Type        string `json:"type"` // deploy | call
	ID          string `json:"id,omitempty"`
	Contract    string `json:"contract,omitempty"`
	ContractRef string `json:"contract_ref,omitempty"`
	From        string `json:"from"`
	Function    string `json:"function,omitempty"`
	Params      []any  `json:"params,omitempty"`
	GasLimit    string `json:"gas_limit,omitempty"`
	Value       string `json:"value,omitempty"`
}

type runtimeContract struct {
	Spec ContractSpec
	ABI  abi.ABI
}

type generator struct {
	cfg Config

	baseDir string
	rows    [][]string

	curBlock int64
	curTS    int64

	nonceBySender         map[string]uint64
	contractRuntimeByName map[string]*runtimeContract
	lastDeployedByName    map[string]common.Address
	deployedByID          map[string]common.Address
	contractAddrSet       map[string]struct{}
}

func main() {
	configPath := flag.String("config", "", "path to scenario json")
	outPath := flag.String("out", "", "override output csv path")
	flag.Parse()

	if *configPath == "" {
		log.Fatal("usage: go run ./cmd/generatecontracttxfile/main.go -config scenario.json [-out output.csv]")
	}

	cfg, baseDir, err := loadConfig(*configPath)
	if err != nil {
		log.Fatalf("load config failed: %v", err)
	}

	if *outPath != "" {
		cfg.Output.CSVPath = *outPath
	}
	if strings.TrimSpace(cfg.Output.CSVPath) == "" {
		log.Fatal("output.csv_path is required or pass -out")
	}

	g, err := newGenerator(cfg, baseDir)
	if err != nil {
		log.Fatalf("init generator failed: %v", err)
	}

	if err = g.generate(); err != nil {
		log.Fatalf("generate failed: %v", err)
	}

	if err = writeCSV(cfg.Output.CSVPath, g.rows); err != nil {
		log.Fatalf("write csv failed: %v", err)
	}

	fmt.Printf("dataset generated: %s (rows=%d)\n", cfg.Output.CSVPath, len(g.rows)-1)
}

func loadConfig(path string) (Config, string, error) {
	f, err := os.Open(path)
	if err != nil {
		return Config{}, "", err
	}
	defer func(f *os.File) {
		_ = f.Close()
	}(f)

	var cfg Config
	dec := json.NewDecoder(f)
	dec.UseNumber()
	if err = dec.Decode(&cfg); err != nil {
		return Config{}, "", err
	}

	if len(cfg.Contracts) == 0 {
		return Config{}, "", errors.New("contracts is empty")
	}
	if len(cfg.Scenarios) == 0 {
		return Config{}, "", errors.New("scenarios is empty")
	}

	if cfg.Output.StartBlockNumber == 0 {
		cfg.Output.StartBlockNumber = 1000010
	}
	if cfg.Output.StartTimestamp == 0 {
		cfg.Output.StartTimestamp = time.Now().Unix()
	}
	if cfg.Output.BlockStep == 0 {
		cfg.Output.BlockStep = 1
	}
	if cfg.Output.TimestampStep == 0 {
		cfg.Output.TimestampStep = 2
	}
	if cfg.Output.DefaultGasPrice == "" {
		cfg.Output.DefaultGasPrice = "0"
	}
	if cfg.Output.DefaultGasUsed == "" {
		cfg.Output.DefaultGasUsed = "0"
	}
	if cfg.Output.DefaultIsError == "" {
		cfg.Output.DefaultIsError = "None"
	}
	if cfg.Output.DefaultEIP2718Type == "" {
		cfg.Output.DefaultEIP2718Type = "None"
	}
	if cfg.Output.DefaultBaseFeePerGas == "" {
		cfg.Output.DefaultBaseFeePerGas = "None"
	}
	if cfg.Output.DefaultMaxFeePerGas == "" {
		cfg.Output.DefaultMaxFeePerGas = "None"
	}
	if cfg.Output.DefaultMaxPriorityFee == "" {
		cfg.Output.DefaultMaxPriorityFee = "None"
	}

	for alias, addr := range cfg.Accounts {
		if !common.IsHexAddress(addr) {
			return Config{}, "", fmt.Errorf("invalid account address %s=%s", alias, addr)
		}
	}

	return cfg, filepath.Dir(path), nil
}

func newGenerator(cfg Config, baseDir string) (*generator, error) {
	g := &generator{
		cfg:                   cfg,
		baseDir:               baseDir,
		rows:                  [][]string{csvHeader},
		curBlock:              cfg.Output.StartBlockNumber,
		curTS:                 cfg.Output.StartTimestamp,
		nonceBySender:         map[string]uint64{},
		contractRuntimeByName: map[string]*runtimeContract{},
		lastDeployedByName:    map[string]common.Address{},
		deployedByID:          map[string]common.Address{},
		contractAddrSet:       map[string]struct{}{},
	}

	for name, c := range cfg.Contracts {
		if strings.TrimSpace(c.ABIPath) == "" {
			return nil, fmt.Errorf("contract %s abi_path is empty", name)
		}
		if strings.TrimSpace(c.Bytecode) == "" {
			return nil, fmt.Errorf("contract %s bytecode is empty", name)
		}
		abiPath := c.ABIPath
		if !filepath.IsAbs(abiPath) {
			abiPath = filepath.Join(baseDir, abiPath)
		}
		b, err := os.ReadFile(abiPath)
		if err != nil {
			return nil, fmt.Errorf("read abi failed (%s): %w", name, err)
		}
		parsed, err := abi.JSON(strings.NewReader(string(b)))
		if err != nil {
			return nil, fmt.Errorf("parse abi failed (%s): %w", name, err)
		}
		g.contractRuntimeByName[name] = &runtimeContract{Spec: c, ABI: parsed}
	}

	return g, nil
}

func (g *generator) generate() error {
	for _, sc := range g.cfg.Scenarios {
		for i, st := range sc.Steps {
			switch strings.ToLower(strings.TrimSpace(st.Type)) {
			case "deploy":
				if err := g.execDeploy(sc.Name, i, st); err != nil {
					return err
				}
			case "call":
				if err := g.execCall(sc.Name, i, st); err != nil {
					return err
				}
			default:
				return fmt.Errorf("scenario %s step %d unknown type: %s", sc.Name, i, st.Type)
			}
		}
	}
	return nil
}

func (g *generator) execDeploy(scn string, idx int, st Step) error {
	rt, ok := g.contractRuntimeByName[st.Contract]
	if !ok {
		return fmt.Errorf("scenario %s step %d: contract not found: %s", scn, idx, st.Contract)
	}

	fromAddr, err := g.resolveAddressToken(st.From)
	if err != nil {
		return fmt.Errorf("scenario %s step %d: bad from: %w", scn, idx, err)
	}

	nonceKey := strings.ToLower(fromAddr.Hex())
	nonce := g.nonceBySender[nonceKey]
	realContractAddr := crypto.CreateAddress(fromAddr, nonce)
	g.nonceBySender[nonceKey] = nonce + 1

	bytecodeHex, err := normalizeHex(rt.Spec.Bytecode)
	if err != nil {
		return fmt.Errorf("scenario %s step %d: bad bytecode: %w", scn, idx, err)
	}

	gasLimit := "2000000"
	if strings.TrimSpace(st.GasLimit) != "" {
		gasLimit = st.GasLimit
	}
	value := "0"
	if strings.TrimSpace(st.Value) != "" {
		if _, ok = new(big.Int).SetString(st.Value, 10); !ok {
			return fmt.Errorf("scenario %s step %d: invalid value: %s", scn, idx, st.Value)
		}
		value = st.Value
	}

	row := g.newBaseRow()
	row[3] = fromAddr.Hex()
	row[4] = "None"
	row[5] = strings.ToLower(realContractAddr.Hex())
	row[6] = bool01(g.isContractAddr(fromAddr))
	row[7] = "0"
	row[8] = value
	row[9] = gasLimit
	row[12] = bytecodeHex
	g.rows = append(g.rows, row)

	g.lastDeployedByName[st.Contract] = realContractAddr
	if strings.TrimSpace(st.ID) != "" {
		g.deployedByID[st.ID] = realContractAddr
	}
	g.contractAddrSet[strings.ToLower(realContractAddr.Hex())] = struct{}{}

	g.step()
	return nil
}

func (g *generator) execCall(scn string, idx int, st Step) error {
	fromAddr, err := g.resolveAddressToken(st.From)
	if err != nil {
		return fmt.Errorf("scenario %s step %d: bad from: %w", scn, idx, err)
	}

	toAddr, rt, err := g.resolveCallTarget(st)
	if err != nil {
		return fmt.Errorf("scenario %s step %d: %w", scn, idx, err)
	}

	method, ok := rt.ABI.Methods[st.Function]
	if !ok {
		return fmt.Errorf("scenario %s step %d: method not found: %s", scn, idx, st.Function)
	}
	if len(st.Params) != len(method.Inputs) {
		return fmt.Errorf("scenario %s step %d: method %s expects %d params, got %d",
			scn, idx, st.Function, len(method.Inputs), len(st.Params))
	}

	args := make([]interface{}, 0, len(st.Params))
	for i, p := range st.Params {
		v, err := g.convertArg(p, method.Inputs[i].Type)
		if err != nil {
			return fmt.Errorf("scenario %s step %d: param[%d] convert failed: %w", scn, idx, i, err)
		}
		args = append(args, v)
	}

	data, err := rt.ABI.Pack(st.Function, args...)
	if err != nil {
		return fmt.Errorf("scenario %s step %d: abi pack failed: %w", scn, idx, err)
	}

	gasLimit := "100000"
	if strings.TrimSpace(st.GasLimit) != "" {
		gasLimit = st.GasLimit
	}
	value := "0"
	if strings.TrimSpace(st.Value) != "" {
		if _, ok = new(big.Int).SetString(st.Value, 10); !ok {
			return fmt.Errorf("scenario %s step %d: invalid value: %s", scn, idx, st.Value)
		}
		value = st.Value
	}

	row := g.newBaseRow()
	row[3] = fromAddr.Hex()
	row[4] = strings.ToLower(toAddr.Hex())
	row[5] = "None"
	row[6] = bool01(g.isContractAddr(fromAddr))
	row[7] = bool01(g.isContractAddr(toAddr))
	row[8] = value
	row[9] = gasLimit
	row[12] = "0x" + hex.EncodeToString(data)
	g.rows = append(g.rows, row)

	g.step()
	return nil
}

func (g *generator) resolveCallTarget(st Step) (common.Address, *runtimeContract, error) {
	var (
		addr common.Address
		ok   bool
	)

	if strings.TrimSpace(st.ContractRef) != "" {
		addr, ok = g.deployedByID[st.ContractRef]
		if !ok {
			return common.Address{}, nil, fmt.Errorf("contract_ref not found: %s", st.ContractRef)
		}
		if strings.TrimSpace(st.Contract) == "" {
			return addr, g.findRuntimeByDeployedAddress(addr), nil
		}
		rt := g.contractRuntimeByName[st.Contract]
		if rt == nil {
			return common.Address{}, nil, fmt.Errorf("contract not found: %s", st.Contract)
		}
		return addr, rt, nil
	}

	if strings.TrimSpace(st.Contract) == "" {
		return common.Address{}, nil, errors.New("call step requires contract or contract_ref")
	}
	rt := g.contractRuntimeByName[st.Contract]
	if rt == nil {
		return common.Address{}, nil, fmt.Errorf("contract not found: %s", st.Contract)
	}
	addr, ok = g.lastDeployedByName[st.Contract]
	if !ok {
		return common.Address{}, nil, fmt.Errorf("contract %s has not been deployed yet", st.Contract)
	}
	return addr, rt, nil
}

func (g *generator) findRuntimeByDeployedAddress(addr common.Address) *runtimeContract {
	// 如果 contract_ref 指向 deploy 步骤但 call 步骤没写 contract，这里按“最近部署记录”回退
	// 简化做法：遍历 lastDeployedByName 匹配地址
	for name, a := range g.lastDeployedByName {
		if strings.EqualFold(a.Hex(), addr.Hex()) {
			return g.contractRuntimeByName[name]
		}
	}
	return nil
}

func (g *generator) resolveAddressToken(token string) (common.Address, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return common.Address{}, errors.New("empty address token")
	}

	if addr, ok := g.cfg.Accounts[token]; ok {
		return common.HexToAddress(addr), nil
	}
	if addr, ok := g.deployedByID[token]; ok {
		return addr, nil
	}
	if common.IsHexAddress(token) {
		return common.HexToAddress(token), nil
	}

	return common.Address{}, fmt.Errorf("cannot resolve address token: %s", token)
}

func (g *generator) convertArg(raw any, t abi.Type) (interface{}, error) {
	switch t.T {
	case abi.AddressTy:
		s, ok := raw.(string)
		if !ok {
			return nil, fmt.Errorf("address must be string token")
		}
		return g.resolveAddressToken(s)

	case abi.UintTy, abi.IntTy:
		return anyToBigInt(raw)

	case abi.BoolTy:
		switch v := raw.(type) {
		case bool:
			return v, nil
		case string:
			lv := strings.ToLower(strings.TrimSpace(v))
			if lv == "true" || lv == "1" {
				return true, nil
			}
			if lv == "false" || lv == "0" {
				return false, nil
			}
			return nil, fmt.Errorf("invalid bool: %s", v)
		default:
			return nil, fmt.Errorf("invalid bool type: %T", raw)
		}

	case abi.StringTy:
		s, ok := raw.(string)
		if !ok {
			return nil, fmt.Errorf("string param must be string")
		}
		return s, nil

	case abi.BytesTy:
		s, ok := raw.(string)
		if !ok {
			return nil, fmt.Errorf("bytes param must be hex string")
		}
		h, err := normalizeHex(s)
		if err != nil {
			return nil, err
		}
		return hex.DecodeString(strings.TrimPrefix(h, "0x"))

	case abi.FixedBytesTy:
		s, ok := raw.(string)
		if !ok {
			return nil, fmt.Errorf("fixed bytes param must be hex string")
		}
		h, err := normalizeHex(s)
		if err != nil {
			return nil, err
		}
		b, err := hex.DecodeString(strings.TrimPrefix(h, "0x"))
		if err != nil {
			return nil, err
		}
		if len(b) > t.Size {
			return nil, fmt.Errorf("fixed bytes too long: %d > %d", len(b), t.Size)
		}
		out := make([]byte, t.Size)
		copy(out[t.Size-len(b):], b)
		return out, nil

	default:
		return nil, fmt.Errorf("unsupported abi type: %s", t.String())
	}
}

func anyToBigInt(v any) (*big.Int, error) {
	switch x := v.(type) {
	case string:
		b, ok := new(big.Int).SetString(strings.TrimSpace(x), 10)
		if !ok {
			return nil, fmt.Errorf("invalid decimal int: %s", x)
		}
		return b, nil
	case json.Number:
		b, ok := new(big.Int).SetString(x.String(), 10)
		if !ok {
			return nil, fmt.Errorf("invalid json number: %s", x.String())
		}
		return b, nil
	case float64:
		// 避免科学计数法大数误差，推荐 JSON 里用字符串传 uint256
		return big.NewInt(int64(x)), nil
	case int:
		return big.NewInt(int64(x)), nil
	case int64:
		return big.NewInt(x), nil
	default:
		return nil, fmt.Errorf("unsupported int type: %T", v)
	}
}

func (g *generator) newBaseRow() []string {
	h := make([]byte, 32)
	_, _ = rand.Read(h)

	return []string{
		fmt.Sprintf("%d", g.curBlock),
		fmt.Sprintf("%d", g.curTS),
		"0x" + hex.EncodeToString(h),
		"", "", "None",
		"0", "0", "0", "100000",
		g.cfg.Output.DefaultGasPrice,
		g.cfg.Output.DefaultGasUsed,
		"None",
		g.cfg.Output.DefaultIsError,
		g.cfg.Output.DefaultEIP2718Type,
		g.cfg.Output.DefaultBaseFeePerGas,
		g.cfg.Output.DefaultMaxFeePerGas,
		g.cfg.Output.DefaultMaxPriorityFee,
	}
}

func (g *generator) step() {
	g.curBlock += g.cfg.Output.BlockStep
	g.curTS += g.cfg.Output.TimestampStep
}

func (g *generator) isContractAddr(addr common.Address) bool {
	_, ok := g.contractAddrSet[strings.ToLower(addr.Hex())]
	return ok
}

func normalizeHex(s string) (string, error) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(strings.TrimPrefix(s, "0x"), "0X")
	if s == "" {
		return "", errors.New("empty hex")
	}
	if len(s)%2 == 1 {
		s = "0" + s
	}
	if _, err := hex.DecodeString(s); err != nil {
		return "", fmt.Errorf("invalid hex: %w", err)
	}
	return "0x" + strings.ToLower(s), nil
}

func bool01(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

func writeCSV(path string, rows [][]string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func(f *os.File) {
		_ = f.Close()
	}(f)

	w := csv.NewWriter(f)
	if err = w.WriteAll(rows); err != nil {
		return err
	}
	w.Flush()
	return w.Error()
}
