package main

import (
	"crypto/rand"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
)

func main() {

	dest := flag.String("out", "", "Path at which to store the marshalled peer identity. Required.")
	flag.Parse()

	if dest == nil || *dest == "" {
		panic("destination path must be specified via --out flag.")
	}

	out := filepath.Clean(*dest)
	k, _, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		panic(err)
	}

	pid, err := peer.IDFromPrivateKey(k)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Generated random private key with peer ID: %s\n", pid)

	mk, err := crypto.MarshalPrivateKey(k)
	if err != nil {
		panic(err)
	}

	if err := os.WriteFile(out, mk, 0666); err != nil {
		panic(err)
	}

	fmt.Printf("Marshalled **unencrypted** private key is stored at: %s\n", out)
}
