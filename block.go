package main

import (
	"time"
	"crypto/sha256"
	"strconv"
	"fmt"
	"bytes"
	"net/http"
	"encoding/json"
	"io/ioutil"
	"io"
	"log"
)

type Block struct {
	Index        int
	Data         string
	Timestamp    int64
	Hash         []byte
	PreviousHash []byte
}

func main() {
	currentTime := time.Now().Unix()
	genesisHash := sha256.New()
	genesisHash.Write([]byte("AceCoinGenesis"))

	genesisBlock := Block{0, "genesis", currentTime, genesisHash.Sum(nil), nil}
	blockchain := make([]Block, 1)
	blockchain[0] = genesisBlock
	fmt.Println("AceCoin successfully initialized.")
	fmt.Println("Genesis:")
	fmt.Println(blockchain)

	// List blocks
	http.HandleFunc("/blocks", func(w http.ResponseWriter, r *http.Request) {
		getBlockchainJSON(blockchain, w)
	})

	// Create new block
	http.HandleFunc("/new", func(w http.ResponseWriter, r *http.Request) {
		setHeaders(&w)
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		body, err := ioutil.ReadAll(io.LimitReader(r.Body, 1048576))
		handleError(err)
		if err := r.Body.Close(); err != nil {
			panic(err)
		}
		var data string
		if err := json.Unmarshal(body, &data); err != nil {
			setHeaders(&w)
			w.WriteHeader(422) // unprocessable entity
			if err := json.NewEncoder(w).Encode(err); err != nil {
				panic(err)
			}
		}
		newBlock := generateNextBlock(&blockchain, blockchain[len(blockchain) - 1], data, w)
		json.NewEncoder(w).Encode(newBlock)
	})

	// Start server
	http.ListenAndServe(":8081", nil)
}

func generateNextBlock(blockchain *[]Block, previousBlock Block, blockData string, w http.ResponseWriter) Block {
	 newBlock := Block{
		Index:        previousBlock.Index + 1,
		Data:         blockData,
		Timestamp:    time.Now().Unix(),
		Hash:         hashBlock(previousBlock),
		PreviousHash: previousBlock.Hash,
	}
	newChain := append(*blockchain, newBlock)
	blockchain = &newChain
	return newBlock
}

func validateNewBlock(newBlock Block, previousBlock Block) bool {
	if newBlock.Index != previousBlock.Index+ 1 {
		fmt.Println("Invalid Index.")
		return false
	}
	if !bytes.Equal(newBlock.PreviousHash, previousBlock.Hash) {
		fmt.Println("Invalid previous hash.")
		return false
	}
	if !bytes.Equal(hashBlock(newBlock), newBlock.Hash) {
		fmt.Println("Invalid block hash.")
		return false
	}
	return true
}

func validateChain(genesisBlock *Block, blockchain *[]Block) bool {
	chainValue := *blockchain
	genesisValue := *genesisBlock
	if !compareBlocks(chainValue[0], genesisValue) {
		return false
	}
	for i := 1; i <= len(chainValue); i++ {
		if !validateNewBlock(chainValue[i + 1], chainValue[i]) {
			return false
		}
	}
	return true
}

func replaceChain(genesisBlock *Block, currentChain *[]Block, currentBlocks []Block, newBlocks []Block) {
	if validateChain(genesisBlock, &newBlocks) && len(newBlocks) > len(currentBlocks) {
		currentChain = &newBlocks
		// broadcast new chain
	} else {
		fmt.Println("Received invalid chain.  Not replacing.")
	}
}

func compareBlocks(blockOne Block, blockTwo Block) bool {
	if blockOne.Index != blockTwo.Index {
		return false
	}
	if blockOne.Data != blockTwo.Data {
		return false
	}
	if blockOne.Timestamp != blockTwo.Timestamp {
		return false
	}
	if !bytes.Equal(blockOne.Hash, blockTwo.Hash) {
		return false
	}
	if !bytes.Equal(blockOne.PreviousHash, blockTwo.PreviousHash) {
		return false
	}
	return true

}

func hashBlock(block Block) []byte {
	hasher := sha256.New()
	hashData := strconv.Itoa(block.Index) + block.Data + strconv.FormatInt(block.Timestamp, 10)
	hasher.Write([]byte(hashData))
	hasher.Write(block.PreviousHash)
	return hasher.Sum(nil)
}

func getBlockchainJSON(blockchain []Block, w http.ResponseWriter) {
	setHeaders(&w)
	json.NewEncoder(w).Encode(blockchain)
}

func setHeaders(w *http.ResponseWriter) {
	(*w).Header().Set("Access-Control-Allow-Origin", "*")
	(*w).Header().Set("Access-Control-Allow-Methods", "*")
	(*w).Header().Set("Access-Control-Allow-Headers", "*")
	(*w).Header().Set("Content-Type", "application/json")
}

func handleError(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
