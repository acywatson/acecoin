package main

import (
	"time"
	"crypto/sha256"
	"strconv"
	"fmt"
	"bytes"
	"net/http"
	"encoding/json"
	"log"
)

type Block struct {
	Index        int    `json:"index"`
	Data         string `json:"data"`
	Timestamp    int64  `json:"timestamp"`
	Hash         []byte `json:"hash"`
	PreviousHash []byte `json:"previousHash"`
}

type UserData struct {
	Data string
}

// The primary chain itself needs to be globally available to avoid convoluted closures in the P2PServer
var blockchain = make([]Block, 1)

func main() {
	// Create genesis block
	genesisTime := time.Now().Unix()
	genesisHash := sha256.New()
	genesisHash.Write([]byte("AceCoinGenesis"))
	genesisBlock := Block{0, "genesis", genesisTime, genesisHash.Sum(nil), nil}

	// Initialize chain and store in memory
	// TODO: Blockchain should implement an interface with all of these validation/generation/replace methods (below)
	// TODO: the Blockchain should have a property called chain of type []Block
	blockchain[0] = genesisBlock
	fmt.Println("AceCoin successfully initialized.")
	fmt.Println("Genesis:")
	fmt.Println(blockchain)

	// Initialize HTTP server
	initializeHttpServer(blockchain)
}

func generateNextBlock(blockchain *[]Block, previousBlock Block, blockData string, w http.ResponseWriter) (Block, *[]Block) {
	 newBlock := Block{
		Index:        previousBlock.Index + 1,
		Data:         blockData,
		Timestamp:    time.Now().Unix(),
		Hash:         hashBlock(previousBlock),
		PreviousHash: previousBlock.Hash,
	}
	newChain := append(*blockchain, newBlock)
	return newBlock, &newChain
}

func validateNewBlock(newBlock Block, previousBlock Block) bool {
	if newBlock.Index != previousBlock.Index + 1 {
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

func validateChain(blockchain *[]Block) bool {
	chainValue := *blockchain
	for i := 1; i <= len(chainValue); i++ {
		if !validateNewBlock(chainValue[i + 1], chainValue[i]) {
			return false
		}
	}
	return true
}

func replaceChain(currentChain *[]Block, currentBlocks []Block, newBlocks []Block) *[]Block {
	if validateChain(&newBlocks) && len(newBlocks) > len(currentBlocks) {
		currentChain = &newBlocks
		return currentChain
	} else {
		fmt.Println("Received invalid chain.  Not replacing.")
		return nil
	}
}

func addBlockToChain(currentChain *[]Block, newBlock Block) *[]Block {
	newChain := append(*currentChain, newBlock)
	currentChain = &newChain
	return currentChain
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

func getLatestBlock(blockchain []Block) Block {
	return blockchain[len(blockchain) - 1]
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
		log.Println(err)
	}
}
