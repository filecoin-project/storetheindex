package command

import (
	"bufio"
	"encoding/json"
	"errors"
	"math/rand"
	"os"
	"time"

	agg "github.com/filecoin-project/go-dagaggregator-unixfs"
	"github.com/ipfs/go-cid"
	"github.com/multiformats/go-multihash"
	"github.com/urfave/cli/v2"
)

const DAG_MAX = 3200000

var SyntheticCmd = &cli.Command{
	Name:   "synthetic",
	Usage:  "Generate synthetic load to import in indexer",
	Flags:  syntheticFlags,
	Action: syntheticCmd,
}

func syntheticCmd(c *cli.Context) error {
	dir := c.String("dir")
	num := c.Int64("num")
	size := c.Int64("size")
	t := c.String("type")
	switch t {
	case "manifest":
		return genManifest(dir, num, size)
	case "cidlist":
		return genCidList(dir, num, size)
	default:
		return errors.New("export type not implemented, try types manifest or cidlist")
	}

}

func genCidList(dir string, num int64, size int64) error {
	log.Infow("Starting to synthetize cidlist file")
	if size != 0 {
		return writeCidFileOfSize(dir, size)
	}
	if num != 0 {
		return writeCidFile(dir, num)
	}

	return errors.New("no size or number of cids provided to command")

}

func genManifest(dir string, num int64, size int64) error {
	log.Infow("Starting to synthetize manifest file")
	if size != 0 {
		return writeManifestOfSize(dir, size)
	}
	if num != 0 {
		return writeManifest(dir, num)
	}
	return errors.New("no size or number of cids provided to command")
}

// writeCidFile creates a file and appends a list of cids.
func writeCidFile(dir string, num int64) error {
	file, err := os.OpenFile(dir,
		os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	w := bufio.NewWriter(file)
	curr := int64(0)
	for curr < num {
		cids, _ := randomCids(10)
		for i := range cids {
			if _, err = w.WriteString(cids[i].String()); err != nil {
				return err
			}
			if _, err = w.WriteString("\n"); err != nil {
				return err
			}
		}
		curr += 10
		progressCids(curr)
	}

	if err = w.Flush(); err != nil {
		return err
	}

	log.Infof("Created cidList successfully")
	return nil
}

// writeCidFileOfSize creates a new file of a specific size
func writeCidFileOfSize(dir string, size int64) error {
	file, err := os.OpenFile(dir,
		os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	w := bufio.NewWriter(file)

	curr := int64(0)
	for curr < size {
		// Generate CIDs in batches of 100
		cids, _ := randomCids(100)
		for _, c := range cids {
			curr += int64(len(c.Bytes()))

			if _, err = w.WriteString(c.String()); err != nil {
				return err
			}
			if _, err = w.WriteString("\n"); err != nil {
				return err
			}
			progressBytes(curr)
		}

	}

	if err = w.Flush(); err != nil {
		return err
	}

	log.Infof("Created cidList successfully of size: %d", size)
	return nil
}

// writeManifest appends new entries to existing manifest
func writeManifest(dir string, num int64) error {
	file, err := os.OpenFile(dir,
		os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	w := bufio.NewWriter(file)

	curr := int64(0)
	for curr < num {
		cids, _ := randomCids(10)
		for i := range cids {
			b, err := manifestEntry(cids[i])
			if err != nil {
				return err
			}
			if _, err = w.Write(b); err != nil {
				return err
			}
			if _, err = w.WriteString("\n"); err != nil {
				return err
			}
		}
		curr += 10
		progressCids(curr)
	}

	if err = w.Flush(); err != nil {
		return err
	}

	log.Infof("Created Manifest successfully")
	return nil
}

// writeManifestOfSize creates a manifest for certain size of CIDs
func writeManifestOfSize(dir string, size int64) error {
	file, err := os.OpenFile(dir,
		os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	w := bufio.NewWriter(file)

	curr := int64(0)
	for curr < size {
		// Generate CIDs in batches of 100
		cids, _ := randomCids(100)
		for _, c := range cids {
			curr += int64(len(c.Bytes()))
			b, err := manifestEntry(c)
			if err != nil {
				return err
			}

			if _, err = w.Write(b); err != nil {
				return err
			}
			if _, err = w.WriteString("\n"); err != nil {
				return err
			}
			progressBytes(curr)
		}

	}

	if err = w.Flush(); err != nil {
		return err
	}

	log.Infof("Created Manifest successfully")
	return nil
}

func manifestEntry(c cid.Cid) ([]byte, error) {
	// NOTE: We are not including ManifestPreamble and Summary,
	// as for importing purposes we only use DagEntries. We are also
	// not setting some of the fields because we currently don't use them
	// for import. Set them conveniently if neccessary.
	n := uint64(1)
	e := agg.ManifestDagEntry{
		RecordType: "DagAggregateEntry",
		NodeCount:  &n,
	}
	switch c.Version() {
	case 1:
		e.DagCidV1 = c.String()
	case 0:
		e.DagCidV0 = c.String()
	default:
		return nil, errors.New("unsupported cid version")
	}

	return json.Marshal(e)
}

func progressBytes(n int64) {
	if n%50000 == 0 {
		log.Infof("Generated %dB so far", n)
	}
}

func progressCids(n int64) {
	if n%1000 == 0 {
		log.Infof("Generated %d cids so far", n)
	}
}

var prefix = cid.Prefix{
	Version:  1,
	Codec:    cid.Raw,
	MhType:   multihash.SHA2_256,
	MhLength: -1, // default length
}

func randomCids(n int) ([]cid.Cid, error) {
	prefix := cid.Prefix{
		Version:  1,
		Codec:    cid.Raw,
		MhType:   multihash.SHA2_256,
		MhLength: -1, // default length
	}

	prng := rand.New(rand.NewSource(time.Now().UnixNano()))

	res := make([]cid.Cid, n)
	for i := 0; i < n; i++ {
		b := make([]byte, 10*n)
		prng.Read(b)
		c, err := prefix.Sum(b)
		if err != nil {
			return nil, err
		}
		res[i] = c
	}
	return res, nil
}
