package primary

import (
	"testing"

	"github.com/filecoin-project/storetheindex/utils"
	peer "github.com/libp2p/go-libp2p-core/peer"
)

var p peer.ID = "12D3KooWKRyzVWW6ChFjQjK4miCty85Niy48tpPV95XdKu1BcvMA"

func TestE2E(t *testing.T) {
	s := New(1000000)
	cids, err := utils.RandomCids(15)
	if err != nil {
		t.Fatal(err)
	}

	piece := cids[0]
	single := cids[1]
	noadd := cids[2]
	batch := cids[3:]

	// Put a single CID
	t.Logf("Put/Get a single CID in primary storage")
	err = s.Put(single, p, piece)
	if err != nil {
		t.Fatal("Error putting single cid: ", err)
	}

	i, found := s.Get(single)
	if !found {
		t.Errorf("Error finding single cid")
	}
	if i[0].PieceID != piece || i[0].ProvID != p {
		t.Errorf("Got wrong value for single cid")
	}

	// Put a batch of CIDs
	t.Logf("Put/Get a batch of CIDs in primary storage")
	err = s.PutMany(batch, p, piece)
	if err != nil {
		t.Fatal("Error putting batch of cids: ", err)
	}

	i, found = s.Get(cids[5])
	if !found {
		t.Errorf("Error finding a cid from the batch")
	}
	if i[0].PieceID != piece || i[0].ProvID != p {
		t.Errorf("Got wrong value for single cid")
	}

	// Put on an existing key
	t.Logf("Put/Get on existing key")
	err = s.Put(single, p, noadd)
	if err != nil {
		t.Fatal("Error putting single cid: ", err)
	}

	i, found = s.Get(single)
	if !found {
		t.Errorf("Error finding a cid from the batch")
	}
	if len(i) != 2 {
		t.Fatal("Update over existing key not correct")
	}
	if i[1].PieceID != noadd || i[1].ProvID != p {
		t.Errorf("Got wrong value for single cid")
	}

	// Get a key that is not set
	t.Logf("Get non-existing key")
	_, found = s.Get(noadd)
	if found {
		t.Errorf("Error, the key for the cid shouldn't be set")
	}
}

func TestRotate(t *testing.T) {
	const maxSize = 10

	cids, err := utils.RandomCids(2)
	if err != nil {
		t.Fatal(err)
	}
	piece := cids[0]
	piece2 := cids[1]

	s := New(maxSize * 2)
	cids, err = utils.RandomCids(maxSize + 5)
	if err != nil {
		t.Fatal(err)
	}

	err = s.PutMany(cids, p, piece)
	if err != nil {
		t.Fatal("Error putting batch of cids: ", err)
	}

	_, found := s.Get(cids[0])
	if !found {
		t.Errorf("Error finding a cid from previous cache")
	}

	_, found = s.Get(cids[maxSize+2])
	if !found {
		t.Errorf("Error finding a cid from new cache")
	}

	cids2, err := utils.RandomCids(maxSize)
	if err != nil {
		t.Fatal(err)
	}

	err = s.PutMany(cids2, p, piece2)
	if err != nil {
		t.Fatal("Error putting batch of cids: ", err)
	}

	// Should find this because it was moved to new cache after 1st rotation
	_, found = s.Get(cids[0])
	if !found {
		t.Errorf("Error finding a cid from previous cache")
	}

	// Should find this because it should be in old cache after 2nd rotation
	_, found = s.Get(cids[maxSize+2])
	if !found {
		t.Errorf("Error finding a cid from new cache")
	}

	// Should not find this because it was only in old cache after 1st rotation
	_, found = s.Get(cids[2])
	if found {
		t.Errorf("cid should have been rotated out of cache")
	}
}
