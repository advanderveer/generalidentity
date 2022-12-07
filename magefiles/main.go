package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/magefile/mage/sh"
)

// init performs some sanity checks before running anything
func init() {
	mustBeInRoot()
	mustHaveSimulatorEnvs()
}

func build() (string, error) {
	run := []string{"main.clsp", "--include", "include", "--strict"}
	clvmb, err := sh.Output("run", run...)
	if err != nil {
		return "", fmt.Errorf("failed to `run %s`: %w", strings.Join(os.Args, " "), err)
	}
	return clvmb, err
}

// puzzle builds the Chia lips into a commitment hash and reveal code
func puzzle() (src, reveal, hash string, err error) {

	// we need to compile our chia lisp to lower-level clvm code
	run := []string{"main.clsp", "--include", "include", "--strict"}
	if src, err = sh.Output("run", run...); err != nil {
		return "", "", "", fmt.Errorf("failed to `run %s`: %w", strings.Join(os.Args, " "), err)
	}

	// to spend it later we need to reveal the puzzle in encoded form
	if reveal, err = sh.Output("opc", src); err != nil {
		return "", "", "", fmt.Errorf("failed to encode as puzzle reveal: %w", err)
	}

	// the puzzle hash will function as the address, a commitment to a future puzzle that can be unlocked
	if hash, err = sh.Output("opc", "-H", src); err != nil {
		return "", "", "", fmt.Errorf("failed to encode puzzle hash: %w", err)
	}

	return
}

// Run compiles and executes our main coin
func Run(env string) error {
	clvmb, err := build()
	if err != nil {
		return fmt.Errorf("failed to build: %w", err)
	}

	return sh.Run("brun", clvmb, env)
}

// fingerprint of wallet that we use for testing
var fingerprint = "4150526850"

// Lock locks up some amount of xch into our coin
func Lock() error {
	psrc, preveal, phash, err := puzzle()
	if err != nil {
		return fmt.Errorf("failed to build puzzle: %w", err)
	}

	fmt.Printf("puzzle source: %s\n", psrc)
	fmt.Printf("puzzle reveal: %s\n", preveal)
	fmt.Printf("puzzle hash: %s\n", phash)

	// encode the puzzle has as an address we can sent funds to
	paddr, err := sh.Output("cdv", "encode", "--prefix", "txch", phash)
	if err != nil {
		return fmt.Errorf("failed to encode puzzle hash as address: %w", err)
	}

	fmt.Printf("puzzle addr: %s\n", paddr)

	// send funds to the address, which are now locked by a password
	if err = sh.Run("chia", "wallet", "send",
		"--amount", "0.01",
		"--fee", "0.00005",
		"--fingerprint", fingerprint,
		"--address", paddr); err != nil {
		return fmt.Errorf("failed to send txch to puzzle address: %w", err)
	}

	return nil
}

type Bundle struct {
	AggregatedSignature string  `json:"aggregated_signature"`
	CoinSpends          []Spend `json:"coin_spends"`
}

type Spend struct {
	Coin         Coin   `json:"coin"`
	PuzzleReveal string `json:"puzzle_reveal"`
	Solution     string `json:"solution"`
}

type Coin struct {
	Amount         int64  `json:"amount"`
	ParentCoinInfo string `json:"parent_coin_info"`
	PuzzleHash     string `json:"puzzle_hash"`
}

// Unlock unlocks the funds and sends the back to the wallet
func Unlock() error {
	_, preveal, phash, err := puzzle()
	if err != nil {
		return fmt.Errorf("failed to build puzzle: %w", err)
	}

	// fetch the coin records
	data, err := sh.Output("cdv", "rpc", "coinrecords", "--only-unspent",
		"--by", "puzzlehash", phash)
	if err != nil {
		return fmt.Errorf("failed to get coin records: %w", err)
	}

	var records []struct {
		Coin Coin `json:"coin"`
	}

	if err = json.Unmarshal([]byte(data), &records); err != nil {
		return fmt.Errorf("failed to decode coin records: %w", err)
	}

	if len(records) < 1 {
		return fmt.Errorf("no records, no funds locked with specified puzzle")
	}

	for i, record := range records {
		fmt.Printf("--- coin #%0.3d ---\n", i)
		fmt.Printf("\tAmount: %d\n", record.Coin.Amount)
		fmt.Printf("\tParent: %v\n", record.Coin.ParentCoinInfo)
		fmt.Printf("\t  Hash: %v\n", record.Coin.PuzzleHash)
	}

	// get our wallet address so we can send the funds back after unlocking the coin
	waddr, err := sh.Output("chia", "wallet", "get_address", "--fingerprint", fingerprint)
	if err != nil {
		return fmt.Errorf("failed to get wallet addr: %w", err)
	}

	fmt.Printf("wallet addr: %s\n", waddr)

	// decode the address because the solution requires a hash value
	whash, err := sh.Output("cdv", "decode", waddr)
	if err != nil {
		return fmt.Errorf("failed to decode wallet addr: %w", err)
	}

	fmt.Printf("wallet hash: %s\n", whash)

	// solution will be passed as input to the puzzle (which we reveal as part), we spend all funds in the coin
	ssolution := fmt.Sprintf(`('foo2' 0x%s %d)`, whash, records[0].Coin.Amount)
	fmt.Printf("solution source: %s\n", ssolution)

	// we need to encode the solution for the tx to be valid
	esolution, err := sh.Output("opc", ssolution)
	if err != nil {
		return fmt.Errorf("failed to encode solution: %w", err)
	}

	fmt.Printf("solution encoded: %s\n", esolution)
	bundle := Bundle{
		AggregatedSignature: "0xc00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
		CoinSpends: []Spend{
			{
				Coin:         records[0].Coin,
				PuzzleReveal: preveal,
				Solution:     esolution,
			},
		},
	}

	bundlef, err := os.Create("spendbundle.json")
	if err != nil {
		return fmt.Errorf("failed to create spend bundle file: %w", err)
	}

	buf := bytes.NewBuffer(nil)
	if err := json.NewEncoder(io.MultiWriter(buf, bundlef)).Encode(bundle); err != nil {
		return fmt.Errorf("failed to encode spend bundle: %w", err)
	}

	fmt.Printf("spend bundle: %s\n", buf.String())
	if err = bundlef.Close(); err != nil {
		return fmt.Errorf("failed to close spend bundle file: %w", err)
	}

	if err = sh.Run("cdv", "rpc", "pushtx", "spendbundle.json"); err != nil {
		return fmt.Errorf("failed to push spend bundle: %w", err)
	}

	return nil
}

// mustBeInRoot checks that the command is run in the project root
func mustBeInRoot() {
	if _, err := os.Stat("go.mod"); err != nil {
		panic("must be in root, couldn't stat go.mod file: " + err.Error())
	}
}

func mustHaveSimulatorEnvs() {
	if keys := os.Getenv("CHIA_KEYS_ROOT"); keys == "" {
		panic("invalid simulator CHIA_KEYS_ROOT, or not set: '" + keys + "'")
	}

	if root := os.Getenv("CHIA_ROOT"); !strings.Contains(root, "simulator") {
		panic("invalid simulator CHIA_ROOT, or not set: '" + root + "'")
	}
}
