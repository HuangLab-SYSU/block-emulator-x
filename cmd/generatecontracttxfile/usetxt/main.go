package main

import (
	"bufio"
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
	"os/exec"
	"path/filepath"
	"strconv"
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

type Program struct {
	Accounts  map[string]string
	Contracts map[string]*ContractDef
	Steps     []Step
}

type ContractDef struct {
	Alias string

	// Used when use-solc=true
	SolPath      string
	ContractName string // optional

	// Used when use-solc=false
	ABIPath      string
	BytecodePath string

	// Runtime fields
	ABI      abi.ABI
	Bytecode string // 0x...
}

type Step struct {
	Scenario    string
	Type        string // DEPLOY / CALL
	ID          string
	Contract    string
	ContractRef string
	From        string
	Function    string
	Params      []string
	GasLimit    string
	Value       string
}

type Generator struct {
	prog *Program
	rows [][]string

	block int64
	ts    int64
	bStep int64
	tStep int64

	nonceBySender     map[string]uint64
	deployedByID      map[string]common.Address
	lastByContract    map[string]common.Address
	deployID2Contract map[string]string
	contractAddrSet   map[string]struct{}
}

type solcCombinedJSON struct {
	Contracts map[string]struct {
		ABI json.RawMessage `json:"abi"`
		Bin string          `json:"bin"`
	} `json:"contracts"`
}

func main() {
	in := flag.String("in", "", "scenario txt path")
	out := flag.String("out", "dataset.csv", "output csv path")
	useSolc := flag.Bool("use-solc", true, "true: compile sol with solc; false: load abi+bytecode from txt paths")

	startBlock := flag.Int64("start-block", 1000010, "start block number")
	startTS := flag.Int64("start-ts", time.Now().Unix(), "start timestamp")
	blockStep := flag.Int64("block-step", 1, "block step")
	tsStep := flag.Int64("ts-step", 3, "timestamp step")

	basePath := flag.String("solc-base-path", "", "solc --base-path (optional)")
	includePath := flag.String("solc-include-path", "", "solc --include-path (optional)")
	optimize := flag.Bool("optimize", true, "solc optimize")

	flag.Parse()

	if strings.TrimSpace(*in) == "" {
		log.Fatal("usage: go run main.go -in scenario.txt -out dataset.csv [-use-solc=true|false]")
	}

	prog, rootDir, err := parseScenarioTXT(*in)
	if err != nil {
		log.Fatal(err)
	}

	for _, c := range prog.Contracts {
		if *useSolc {
			if err = compileWithSolc(c, rootDir, *basePath, *includePath, *optimize); err != nil {
				log.Fatalf("compile %s failed: %v", c.Alias, err)
			}
		} else {
			if err = loadArtifacts(c, rootDir); err != nil {
				log.Fatalf("load artifacts %s failed: %v", c.Alias, err)
			}
		}
	}

	g := &Generator{
		prog:              prog,
		rows:              [][]string{csvHeader},
		block:             *startBlock,
		ts:                *startTS,
		bStep:             *blockStep,
		tStep:             *tsStep,
		nonceBySender:     map[string]uint64{},
		deployedByID:      map[string]common.Address{},
		lastByContract:    map[string]common.Address{},
		deployID2Contract: map[string]string{},
		contractAddrSet:   map[string]struct{}{},
	}
	if err = g.run(); err != nil {
		log.Fatal(err)
	}

	if err = writeCSV(*out, g.rows); err != nil {
		log.Fatal(err)
	}

	fmt.Printf("dataset generated: %s (rows=%d)\n", *out, len(g.rows)-1)
}

func parseScenarioTXT(path string) (*Program, string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, "", err
	}
	defer func(f *os.File) {
		_ = f.Close()
	}(f)

	p := &Program{
		Accounts:  map[string]string{},
		Contracts: map[string]*ContractDef{},
		Steps:     []Step{},
	}
	curScenario := "default"
	lineNo := 0

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		lineNo++

		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		fields := strings.Fields(line)
		cmd := strings.ToUpper(fields[0])

		switch cmd {
		case "SCENARIO":
			if len(fields) < 2 {
				return nil, "", fmt.Errorf("line %d: SCENARIO missing name", lineNo)
			}

			curScenario = fields[1]

		case "ACCOUNT":
			// ACCOUNT alice 0x...
			if len(fields) != 3 {
				return nil, "", fmt.Errorf("line %d: ACCOUNT format error", lineNo)
			}

			if !common.IsHexAddress(fields[2]) {
				return nil, "", fmt.Errorf("line %d: invalid account address %s", lineNo, fields[2])
			}

			p.Accounts[fields[1]] = strings.ToLower(fields[2])

		case "CONTRACT":
			// use-solc=true:
			// CONTRACT Token sol=./contracts/Token.sol name=Token
			// use-solc=false:
			// CONTRACT Token abi=./abi/Token.json bytecode=./bin/Token.bin
			if len(fields) < 3 {
				return nil, "", fmt.Errorf("line %d: CONTRACT format error", lineNo)
			}

			alias := fields[1]
			kv := parseKV(fields[2:])
			p.Contracts[alias] = &ContractDef{
				Alias:        alias,
				SolPath:      kv["sol"],
				ContractName: kv["name"],
				ABIPath:      kv["abi"],
				BytecodePath: kv["bytecode"],
			}

		case "DEPLOY", "CALL":
			kv := parseKV(fields[1:])

			st := Step{
				Scenario:    curScenario,
				Type:        cmd,
				ID:          kv["id"],
				Contract:    kv["contract"],
				ContractRef: kv["contract_ref"],
				From:        kv["from"],
				Function:    kv["function"],
				GasLimit:    kv["gas"],
				Value:       kv["value"],
			}
			if kv["params"] != "" {
				st.Params = splitParams(kv["params"])
			}

			p.Steps = append(p.Steps, st)

		default:
			return nil, "", fmt.Errorf("line %d: unknown command %s", lineNo, cmd)
		}
	}

	if err = sc.Err(); err != nil {
		return nil, "", err
	}

	return p, filepath.Dir(path), nil
}

func compileWithSolc(c *ContractDef, rootDir, basePath, includePath string, optimize bool) error {
	if strings.TrimSpace(c.SolPath) == "" {
		return fmt.Errorf("contract %s requires sol=... when -use-solc=true", c.Alias)
	}

	solPath := c.SolPath
	if !filepath.IsAbs(solPath) {
		solPath = filepath.Join(rootDir, solPath)
	}

	args := []string{}
	if optimize {
		args = append(args, "--optimize")
	}

	if strings.TrimSpace(basePath) != "" {
		args = append(args, "--base-path", basePath)
	}

	if strings.TrimSpace(includePath) != "" {
		args = append(args, "--include-path", includePath)
	}

	args = append(args, "--combined-json", "abi,bin", solPath)

	out, err := exec.Command("solc", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("solc failed: %w, output=%s", err, string(out))
	}

	var cj solcCombinedJSON
	if err = json.Unmarshal(out, &cj); err != nil {
		return fmt.Errorf("parse solc output failed: %w", err)
	}

	targetKey, err := selectContractKey(c, cj.Contracts)
	if err != nil {
		return err
	}

	item := cj.Contracts[targetKey]

	abiText, err := normalizeABIText(item.ABI)
	if err != nil {
		return fmt.Errorf("normalize abi failed: %w", err)
	}

	if strings.TrimSpace(item.Bin) == "" {
		return fmt.Errorf("empty bin for contract key %s", targetKey)
	}

	parsedABI, err := abi.JSON(strings.NewReader(abiText))
	if err != nil {
		return fmt.Errorf("parse abi failed: %w", err)
	}

	c.ABI = parsedABI
	c.Bytecode = "0x" + strings.TrimPrefix(strings.ToLower(strings.TrimSpace(item.Bin)), "0x")

	return nil
}

func selectContractKey(c *ContractDef, contracts map[string]struct {
	ABI json.RawMessage `json:"abi"`
	Bin string          `json:"bin"`
},
) (string, error) {
	// Keep only deployable contracts (non-empty bin)
	keys := make([]string, 0, len(contracts))
	for k, v := range contracts {
		if strings.TrimSpace(v.Bin) != "" {
			keys = append(keys, k)
		}
	}

	if len(keys) == 0 {
		return "", errors.New("no deployable contract found (bin empty)")
	}

	// Explicit contract name from txt
	if strings.TrimSpace(c.ContractName) != "" {
		suffix := ":" + c.ContractName
		for _, k := range keys {
			if strings.HasSuffix(k, suffix) {
				return k, nil
			}
		}

		return "", fmt.Errorf("contract %s not found in solc output", c.ContractName)
	}

	// Auto-select by alias
	suffixAlias := ":" + c.Alias
	for _, k := range keys {
		if strings.HasSuffix(k, suffixAlias) {
			return k, nil
		}
	}

	// Use directly if there is only one candidate
	if len(keys) == 1 {
		return keys[0], nil
	}

	return "", fmt.Errorf(
		"multiple deployable contracts found, please set name=... in CONTRACT line; candidates=%v",
		keys,
	)
}

func loadArtifacts(c *ContractDef, rootDir string) error {
	if strings.TrimSpace(c.ABIPath) == "" || strings.TrimSpace(c.BytecodePath) == "" {
		return fmt.Errorf("contract %s requires abi=... and bytecode=... when -use-solc=false", c.Alias)
	}

	abiPath := c.ABIPath
	if !filepath.IsAbs(abiPath) {
		abiPath = filepath.Join(rootDir, abiPath)
	}

	abiBytes, err := os.ReadFile(abiPath)
	if err != nil {
		return fmt.Errorf("read abi failed: %w", err)
	}

	abiText := strings.TrimSpace(string(abiBytes))
	// Support artifact object: {"abi":[...]}
	if strings.HasPrefix(abiText, "{") {
		var obj struct {
			ABI json.RawMessage `json:"abi"`
		}
		if err = json.Unmarshal(abiBytes, &obj); err != nil {
			return fmt.Errorf("parse abi json failed: %w", err)
		}

		abiText = strings.TrimSpace(string(obj.ABI))
	}

	parsedABI, err := abi.JSON(strings.NewReader(abiText))
	if err != nil {
		return fmt.Errorf("parse abi failed: %w", err)
	}

	binPath := c.BytecodePath
	if !filepath.IsAbs(binPath) {
		binPath = filepath.Join(rootDir, binPath)
	}

	binBytes, err := os.ReadFile(binPath)
	if err != nil {
		return fmt.Errorf("read bytecode failed: %w", err)
	}

	bin, err := normalizeHex(string(binBytes))
	if err != nil {
		return fmt.Errorf("invalid bytecode: %w", err)
	}

	c.ABI = parsedABI
	c.Bytecode = bin

	return nil
}

func normalizeABIText(raw json.RawMessage) (string, error) {
	s := strings.TrimSpace(string(raw))
	if s == "" || s == "null" {
		return "", errors.New("empty abi")
	}
	// ABI is a JSON string
	if strings.HasPrefix(s, "\"") {
		var unq string
		if err := json.Unmarshal(raw, &unq); err != nil {
			return "", err
		}

		unq = strings.TrimSpace(unq)
		if unq == "" {
			return "", errors.New("empty abi string")
		}

		return unq, nil
	}
	// ABI is an array/object directly
	if strings.HasPrefix(s, "[") || strings.HasPrefix(s, "{") {
		return s, nil
	}

	return "", fmt.Errorf("unknown abi format: %s", s)
}

func (g *Generator) run() error {
	for i, st := range g.prog.Steps {
		switch st.Type {
		case "DEPLOY":
			if err := g.execDeploy(st); err != nil {
				return fmt.Errorf("step %d DEPLOY failed: %w", i+1, err)
			}
		case "CALL":
			if err := g.execCall(st); err != nil {
				return fmt.Errorf("step %d CALL failed: %w", i+1, err)
			}
		default:
			return fmt.Errorf("step %d unknown type: %s", i+1, st.Type)
		}
	}

	return nil
}

func (g *Generator) execDeploy(st Step) error {
	c := g.prog.Contracts[st.Contract]
	if c == nil {
		return fmt.Errorf("unknown contract: %s", st.Contract)
	}

	from, err := g.resolveAddress(st.From)
	if err != nil {
		return err
	}

	senderKey := strings.ToLower(from.Hex())
	nonce := g.nonceBySender[senderKey]
	contractAddr := crypto.CreateAddress(from, nonce)
	g.nonceBySender[senderKey] = nonce + 1

	row := g.baseRow()
	row[3] = from.Hex()
	row[4] = "None"
	row[5] = strings.ToLower(contractAddr.Hex())
	row[6] = bool01(g.isContract(from))
	row[7] = "0"
	row[8] = defaultStr(st.Value, "0")
	row[9] = defaultStr(st.GasLimit, "2000000")
	row[12] = c.Bytecode
	g.rows = append(g.rows, row)

	g.lastByContract[st.Contract] = contractAddr

	g.contractAddrSet[strings.ToLower(contractAddr.Hex())] = struct{}{}
	if st.ID != "" {
		g.deployedByID[st.ID] = contractAddr
		g.deployID2Contract[st.ID] = st.Contract
	}

	g.step()

	return nil
}

func (g *Generator) execCall(st Step) error {
	from, err := g.resolveAddress(st.From)
	if err != nil {
		return err
	}

	var (
		to          common.Address
		contractKey string
	)

	if st.ContractRef != "" {
		addr, ok := g.deployedByID[st.ContractRef]
		if !ok {
			return fmt.Errorf("unknown contract_ref: %s", st.ContractRef)
		}

		to = addr
		contractKey = g.deployID2Contract[st.ContractRef]
	}

	if st.Contract != "" {
		contractKey = st.Contract
		if to == (common.Address{}) {
			addr, ok := g.lastByContract[st.Contract]
			if !ok {
				return fmt.Errorf("contract %s not deployed yet", st.Contract)
			}

			to = addr
		}
	}

	if contractKey == "" {
		return errors.New("CALL requires contract or contract_ref")
	}

	c := g.prog.Contracts[contractKey]
	if c == nil {
		return fmt.Errorf("unknown contract: %s", contractKey)
	}

	method, ok := c.ABI.Methods[st.Function]
	if !ok {
		return fmt.Errorf("method not found: %s.%s", contractKey, st.Function)
	}

	if len(method.Inputs) != len(st.Params) {
		return fmt.Errorf(
			"%s.%s params mismatch, want=%d got=%d",
			contractKey,
			st.Function,
			len(method.Inputs),
			len(st.Params),
		)
	}

	args := make([]interface{}, 0, len(st.Params))
	for i := range st.Params {
		val, err := g.convertArg(st.Params[i], method.Inputs[i].Type)
		if err != nil {
			return fmt.Errorf("param[%d] convert failed: %w", i, err)
		}

		args = append(args, val)
	}

	data, err := c.ABI.Pack(st.Function, args...)
	if err != nil {
		return fmt.Errorf("abi pack failed: %w", err)
	}

	row := g.baseRow()
	row[3] = from.Hex()
	row[4] = strings.ToLower(to.Hex())
	row[5] = "None"
	row[6] = bool01(g.isContract(from))
	row[7] = "1"
	row[8] = defaultStr(st.Value, "0")
	row[9] = defaultStr(st.GasLimit, "100000")
	row[12] = "0x" + hex.EncodeToString(data)
	g.rows = append(g.rows, row)

	g.step()

	return nil
}

func (g *Generator) convertArg(raw string, t abi.Type) (interface{}, error) {
	switch t.T {
	case abi.AddressTy:
		return g.resolveAddress(raw)
	case abi.UintTy, abi.IntTy:
		v, ok := new(big.Int).SetString(strings.TrimSpace(raw), 10)
		if !ok {
			return nil, fmt.Errorf("invalid int: %s", raw)
		}

		return v, nil
	case abi.BoolTy:
		s := strings.ToLower(strings.TrimSpace(raw))
		return s == "true" || s == "1", nil
	case abi.StringTy:
		return raw, nil
	case abi.BytesTy:
		h, err := normalizeHex(raw)
		if err != nil {
			return nil, err
		}

		return hex.DecodeString(strings.TrimPrefix(h, "0x"))
	default:
		return nil, fmt.Errorf("unsupported abi type: %s", t.String())
	}
}

func (g *Generator) resolveAddress(token string) (common.Address, error) {
	token = strings.TrimSpace(token)

	if v, ok := g.prog.Accounts[token]; ok {
		return common.HexToAddress(v), nil
	}

	if v, ok := g.deployedByID[token]; ok {
		return v, nil
	}

	if common.IsHexAddress(token) {
		return common.HexToAddress(token), nil
	}

	return common.Address{}, fmt.Errorf("cannot resolve address token: %s", token)
}

func (g *Generator) isContract(addr common.Address) bool {
	_, ok := g.contractAddrSet[strings.ToLower(addr.Hex())]
	return ok
}

func (g *Generator) baseRow() []string {
	hash := make([]byte, 32)
	_, _ = rand.Read(hash)

	return []string{
		strconv.FormatInt(g.block, 10),
		strconv.FormatInt(g.ts, 10),
		"0x" + hex.EncodeToString(hash),
		"", "", "None",
		"0", "0",
		"0", "100000", // value, gasLimit (can be overridden by step)
		"0", "0",
		"None",
		"None", "None", "None", "None", "None",
	}
}

func (g *Generator) step() {
	g.block += g.bStep
	g.ts += g.tStep
}

func parseKV(parts []string) map[string]string {
	m := map[string]string{}

	for _, p := range parts {
		kv := strings.SplitN(p, "=", 2)
		if len(kv) == 2 {
			k := strings.ToLower(strings.TrimSpace(kv[0]))
			v := strings.TrimSpace(kv[1])
			m[k] = v
		}
	}

	return m
}

func splitParams(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}

	items := strings.Split(s, "|")

	out := make([]string, 0, len(items))
	for _, it := range items {
		out = append(out, strings.TrimSpace(it))
	}

	return out
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
		return "", err
	}

	return "0x" + strings.ToLower(s), nil
}

func bool01(b bool) string {
	if b {
		return "1"
	}

	return "0"
}

func defaultStr(s, d string) string {
	if strings.TrimSpace(s) == "" {
		return d
	}

	return s
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
