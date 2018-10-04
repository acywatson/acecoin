package main

import (
	"net/http"
	"io/ioutil"
	"io"
	"encoding/json"
)

func initializeHttpServer(blockchain []Block) {

	// Start P2P server
	p2pServer := getNewP2PServer()
	go initializeP2PServer(p2pServer)

	// List blocks
	http.HandleFunc("/chain", func(w http.ResponseWriter, r *http.Request) {
		getBlockchainJSON(blockchain, w)
	})

	// Create new block
	http.HandleFunc("/addBlock", func(w http.ResponseWriter, r *http.Request) {
		setHeaders(&w)
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		body, err := ioutil.ReadAll(io.LimitReader(r.Body, 1048576))
		handleError(err)
		if err := r.Body.Close(); err != nil {
			handleError(err)
			panic(err)
		}
		var data UserData
		if err := json.Unmarshal(body, &data); err != nil {
			setHeaders(&w)
			w.WriteHeader(422) // unprocessable entity
			handleError(err)
			if err := json.NewEncoder(w).Encode(err); err != nil {
				handleError(err)
				panic(err)
			}
			return
		}
		newBlock, newChain := generateNextBlock(&blockchain, blockchain[len(blockchain) - 1], data.Data, w)
		blockchain = *newChain
		// fmt.Println("New Chain:")
		// fmt.Println(blockchain)
		json.NewEncoder(w).Encode(newBlock)
	})

	// P2P Endpoints //

	// List peers
	http.HandleFunc("/peers", func(w http.ResponseWriter, r *http.Request) {
		// List peers
	})

	// Add peer connection
	http.HandleFunc("/addPeer", func(w http.ResponseWriter, r *http.Request) {
		serveWs(p2pServer, w, r)
	})

	// Start server
	http.ListenAndServe(":8081", nil)
}