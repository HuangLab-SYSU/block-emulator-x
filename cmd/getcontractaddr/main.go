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
	overwrite := flag.Bool("overwrite", true, "overwrite existing toCreate value")
	inPlace := flag.Bool("in-place", false, "modify input file in place")

	flag.Parse()

	if *inPath == "" {
		log.Fatal("usage: go run cmd/getcontractaddr/main.go -in input.csv [-out output.csv] [--in-place]")
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

	// sender -> deploy count as nonce
	sender2nonce := map[string]uint64{}
	filled := 0

	for i := 1; i < len(rows); i++ {
		row := rows[i]

		from := strings.TrimSpace(row[idx["from"]])
		to := strings.TrimSpace(row[idx["to"]])
		toCreate := strings.TrimSpace(row[idx["tocreate"]])
		data := strings.TrimSpace(row[idx["callingfunction"]])

		isDeploy := isEmptyLike(to) && !isEmptyLike(data) && !strings.EqualFold(data, "0x")
		if !isDeploy {
			continue
		}

		if !*overwrite && !isEmptyLike(toCreate) {
			continue
		}

		if !common.IsHexAddress(from) {
			log.Fatalf("row %d invalid from address: %s", i+1, from)
		}

		key := strings.ToLower(from)
		nonce := sender2nonce[key]
		addr := crypto.CreateAddress(common.HexToAddress(from), nonce)

		row[idx["tocreate"]] = strings.ToLower(addr.Hex())
		sender2nonce[key] = nonce + 1
		filled++
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

	fmt.Printf("done, filled toCreate for %d deploy txs\n", filled)
}

func isEmptyLike(s string) bool {
	s = strings.TrimSpace(s)
	return s == "" || strings.EqualFold(s, "none")
}
