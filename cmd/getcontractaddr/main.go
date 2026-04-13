package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

func main() {
	inPath := flag.String("in", "", "input csv path")
	outPath := flag.String("out", "", "output csv path")
	overwrite := flag.Bool("overwrite", true, "overwrite existing toCreate/to")
	inPlace := flag.Bool("in-place", false, "modify input file in place")

	flag.Parse()

	if *inPath == "" {
		log.Fatal("usage: go run cmd/getcontractaddr/main.go -in input.csv [-out output.csv] [-in-place=true]")
	}

	if !*inPlace && *outPath == "" {
		log.Fatal("when --in-place is false, -out is required")
	}

	if *inPlace && *outPath != "" {
		log.Fatal("do not use -out together with --in-place")
	}

	inF, err := os.Open(*inPath)
	if err != nil {
		log.Fatalf("open input failed: %v", err)
	}
	defer func(inF *os.File) {
		_ = inF.Close()
	}(inF)

	r := csv.NewReader(inF)

	rows, err := r.ReadAll()
	if err != nil {
		log.Fatalf("read csv failed: %v", err)
	}

	if len(rows) == 0 {
		log.Fatal("empty csv")
	}

	header := rows[0]

	idx := map[string]int{}
	for i, h := range header {
		idx[strings.ToLower(strings.TrimSpace(h))] = i
	}

	required := []string{"from", "to", "tocreate", "callingfunction"}
	for _, k := range required {
		if _, ok := idx[k]; !ok {
			log.Fatalf("missing required column: %s", k)
		}
	}

	// sender2nonce tracks CREATE nonce progression per sender address.
	sender2nonce := map[string]uint64{}
	// placeholder2real maps original placeholder address to computed contract address.
	placeholder2real := map[string]string{}

	deployFilled := 0
	callToFilled := 0

	for i := 1; i < len(rows); i++ {
		row := rows[i]

		from := strings.TrimSpace(row[idx["from"]])
		to := strings.TrimSpace(row[idx["to"]])
		toCreate := strings.TrimSpace(row[idx["tocreate"]])
		data := strings.TrimSpace(row[idx["callingfunction"]])

		// First pass in this row: if it's a call tx and its "to" matches a known placeholder,
		// replace it with the computed real contract address.
		toKey := normAddrKey(to)
		if toKey != "" {
			if realAddr, ok := placeholder2real[toKey]; ok {
				if *overwrite || isEmptyLike(to) || strings.EqualFold(to, toKey) {
					row[idx["to"]] = realAddr
					callToFilled++
				}
			}
		}

		// A deployment tx is identified by empty "to" + non-empty bytecode in callingFunction.
		isDeploy := isEmptyLike(to) && !isEmptyLike(data) && !strings.EqualFold(data, "0x")
		if !isDeploy {
			continue
		}

		if !common.IsHexAddress(from) {
			log.Fatalf("row %d invalid from address: %s", i+1, from)
		}

		key := strings.ToLower(from)
		nonce := sender2nonce[key]
		addr := strings.ToLower(crypto.CreateAddress(common.HexToAddress(from), nonce).Hex())

		// Record mapping from original toCreate placeholder to the real deployed address.
		placeholderKey := normAddrKey(toCreate)
		if placeholderKey != "" {
			placeholder2real[placeholderKey] = addr
		}

		// Fill deploy tx "toCreate" with computed address.
		if *overwrite || isEmptyLike(toCreate) || !common.IsHexAddress(toCreate) {
			row[idx["tocreate"]] = addr
		}

		deployFilled++

		sender2nonce[key] = nonce + 1
	}

	targetPath := *outPath
	if *inPlace {
		targetPath = *inPath
	}

	outF, err := os.Create(targetPath)
	if err != nil {
		log.Fatalf("create output failed: %v", err)
	}
	defer func(outF *os.File) {
		_ = outF.Close()
	}(outF)

	w := csv.NewWriter(outF)
	if err = w.WriteAll(rows); err != nil {
		log.Fatalf("write csv failed: %v", err)
	}

	w.Flush()

	if err = w.Error(); err != nil {
		log.Fatalf("flush csv failed: %v", err)
	}

	fmt.Printf("done, deploy filled: %d, call.to filled: %d\n", deployFilled, callToFilled)
}

func isEmptyLike(s string) bool {
	s = strings.TrimSpace(s)
	return s == "" || strings.EqualFold(s, "none")
}

func normAddrKey(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	if isEmptyLike(s) {
		return ""
	}
	// Keep non-0x placeholders as valid map keys as well.
	if strings.HasPrefix(s, "0x") {
		return s
	}

	return s
}
