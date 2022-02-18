package ingest

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	indexer "github.com/filecoin-project/go-indexer-core"
	coremetrics "github.com/filecoin-project/go-indexer-core/metrics"
	"github.com/filecoin-project/go-legs"
	"github.com/filecoin-project/storetheindex/api/v0/ingest/schema"
	"github.com/filecoin-project/storetheindex/config"
	"github.com/filecoin-project/storetheindex/internal/registry"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/query"
	logging "github.com/ipfs/go-log/v2"
	"github.com/ipld/go-ipld-prime/datamodel"
	"github.com/ipld/go-ipld-prime/node/basicnode"
	"github.com/ipld/go-ipld-prime/traversal/selector/builder"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/multiformats/go-multiaddr"
	"go.opencensus.io/stats"
)

var log = logging.Logger("indexer/ingest")

// prefix used to track latest sync in datastore.
const (
	syncPrefix  = "/sync/"
	admapPrefix = "/admap/"
	// This prefix represents all the ads we've already processed.
	adProcessedPrefix = "/adProcessed/"
)

// Ingester is a type that uses go-legs for the ingestion protocol.
type Ingester struct {
	host    host.Host
	ds      datastore.Batching
	indexer indexer.Interface

	batchSize int
	closeOnce sync.Once
	sigUpdate chan struct{}

	sub         *legs.Subscriber
	syncTimeout time.Duration

	adCache      map[cid.Cid]schema.Advertisement
	adCacheMutex sync.Mutex

	entriesSel datamodel.Node
	reg        *registry.Registry

	skips      map[string]struct{}
	skipsMutex sync.Mutex

	// inEvents is used to send a legs.SyncFinished to the distributeEvents
	// goroutine, when an advertisement in marked complete.
	inEvents chan legs.SyncFinished

	// outEventsChans is a slice of channels, where each channel delivers a
	// copy of a legs.SyncFinished to an onAdProcessed reader.
	outEventsChans map[peer.ID][]chan cid.Cid
	outEventsMutex sync.Mutex

	waitForPendingSyncs sync.WaitGroup
	closePendingSyncs   chan struct{}

	// staging area
	// We don't need a mutex here because only one goroutine will ever touch this.
	stagingProviderAds map[peer.ID][]cidsWithPublisher

	providersBeingProcessedMu sync.Mutex
	providersBeingProcessed   map[peer.ID]bool
	toWorkers                 chan toWorkerMsg
	closeWorkers              chan struct{}
	waitForWorkers            sync.WaitGroup
	toStaging                 chan legs.SyncFinished
}

// NewIngester creates a new Ingester that uses a go-legs Subscriber to handle
// communication with providers.
func NewIngester(cfg config.Ingest, h host.Host, idxr indexer.Interface, reg *registry.Registry, ds datastore.Batching) (*Ingester, error) {
	// Cleanup any leftover entry cid to ad cid mappings.
	err := removeEntryAdMappings(context.Background(), ds)
	if err != nil {
		log.Errorw("Error cleaning temporary entries ad to mappings", "err", err)
		// Do not return error; keep going.
	}

	lsys := mkLinkSystem(ds)

	// Construct a selector that recursively looks for nodes with field
	// "PreviousID" as per Advertisement schema.  Note that the entries within
	// an advertisement are synced separately triggered by storage hook, so
	// that we can check if a chain of chunks exist already before syncing it.
	ssb := builder.NewSelectorSpecBuilder(basicnode.Prototype.Any)
	adSel := ssb.ExploreFields(
		func(efsb builder.ExploreFieldsSpecBuilder) {
			efsb.Insert("PreviousID", ssb.ExploreRecursiveEdge())
		}).Node()

	// Construct the selector used when syncing entries of an advertisement with the configured
	// recursion limit.
	entSel := ssb.ExploreRecursive(cfg.EntriesRecursionLimit(),
		ssb.ExploreFields(func(efsb builder.ExploreFieldsSpecBuilder) {
			efsb.Insert("Next", ssb.ExploreRecursiveEdge()) // Next field in EntryChunk
		})).Node()

	ing := &Ingester{
		host:        h,
		ds:          ds,
		indexer:     idxr,
		batchSize:   cfg.StoreBatchSize,
		sigUpdate:   make(chan struct{}, 1),
		syncTimeout: time.Duration(cfg.SyncTimeout),
		entriesSel:  entSel,
		reg:         reg,
		inEvents:    make(chan legs.SyncFinished, 1),

		closePendingSyncs: make(chan struct{}),

		stagingProviderAds: make(map[peer.ID][]cidsWithPublisher),

		providersBeingProcessed: make(map[peer.ID]bool),
		toWorkers:               make(chan toWorkerMsg),
		closeWorkers:            make(chan struct{}),
		toStaging:               make(chan legs.SyncFinished, 1),
	}

	// Create and start pubsub subscriber.  This also registers the storage
	// hook to index data as it is received.
	sub, err := legs.NewSubscriber(h, ds, lsys, cfg.PubSubTopic, adSel, legs.AllowPeer(reg.Authorized), legs.SyncRecursionLimit(cfg.AdvertisementRecursionLimit()))
	if err != nil {
		log.Errorw("Failed to start pubsub subscriber", "err", err)
		return nil, errors.New("ingester subscriber failed")
	}
	ing.sub = sub

	err = ing.restoreLatestSync()
	if err != nil {
		sub.Close()
		return nil, err
	}

	go ing.loopIngester(2)

	// TODO handle cancel
	onsyncFinishedChan, _ := ing.sub.OnSyncFinished()
	go func() {
		for syncFinished := range onsyncFinishedChan {
			fmt.Println("Sending to staging")
			ing.toStaging <- syncFinished
			fmt.Println("done Sending to staging")
			// syncFinished.SyncedCids
			// for i := len(syncFinished.SyncedCids) - 1; i >= 0; i-- {
			// 	adCid := syncFinished.SyncedCids[i]
			// 	node, err := lsys.Load(linking.LinkContext{}, cidlink.Link{Cid: adCid}, basicnode.Prototype.Any)
			// 	if err != nil {
			// 		fmt.Println("!!!!!!! Error loading advertisement:", err)
			// 		continue
			// 	}

			// 	ad, err := decodeAd(node)
			// 	if err != nil {
			// 		fmt.Println("!!!!! Error decoding advertisement:", err)
			// 		continue
			// 	}

			// 	fmt.Println("PROCESSING ADVERTISEMENT:", adCid)
			// 	// cid.Undef so we avoid cidwaiter code
			// 	ing.syncAdEntries(syncFinished.PeerID, ad, adCid, cid.Undef)
			// 	fmt.Println("DONE PROCESSING ADVERTISEMENT:", adCid)
			// }
		}

	}()

	// Start distributor to send SyncFinished messages to interested parties.
	go ing.distributeEvents()

	go ing.metricsUpdater()

	log.Debugf("Ingester started and all hooks and linksystem registered")

	return ing, nil
}

func (ing *Ingester) Close() error {
	// Close leg transport.
	err := ing.sub.Close()

	// Dismiss any event readers.
	ing.outEventsMutex.Lock()
	for _, chans := range ing.outEventsChans {
		for _, ch := range chans {
			close(ch)
		}
	}
	ing.outEventsChans = nil
	ing.outEventsMutex.Unlock()

	ing.closeOnce.Do(func() {
		close(ing.closeWorkers)
		ing.waitForWorkers.Wait()
		fmt.Println("Waiting for pending sync")
		close(ing.closePendingSyncs)
		ing.waitForPendingSyncs.Wait()
		fmt.Println("Done waiting for pending sync")

		// Stop the distribution goroutine.
		close(ing.inEvents)

		close(ing.sigUpdate)
	})

	return err
}

// Sync syncs the latest advertisement from a publisher.  This is done by first
// fetching the latest advertisement ID from and traversing it until traversal
// gets to the last seen advertisement.  Then the entries in each advertisement
// are synced and the multihashes in each entry are indexed.
//
// The Context argument controls the lifetime of the sync.  Canceling it
// cancels the sync and causes the multihash channel to close without any data.
//
// Note that the multihash entries corresponding to the advertisement are
// synced in the background.  The completion of advertisement sync does not
// necessarily mean that the entries corresponding to the advertisement are
// synced.
func (ing *Ingester) Sync(ctx context.Context, peerID peer.ID, peerAddr multiaddr.Multiaddr) (<-chan cid.Cid, error) {
	out := make(chan cid.Cid, 1)

	ing.waitForPendingSyncs.Add(1)
	go func() {
		defer ing.waitForPendingSyncs.Done()
		defer close(out)

		log := log.With("peerID", peerID)
		log.Debug("Explicitly syncing the latest advertisement from peer")

		syncDone, cancel := ing.onAdProcessed(peerID)
		defer cancel()

		latest, err := ing.GetLatestSync(peerID)
		if err != nil {
			log.Errorw("Failed to get latest sync", "err", err)
			return
		}

		// Start syncing. Notifications for the finished sync are sent
		// asynchronously.  Sync with cid.Undef and a nil selector so that:
		//
		//   1. The latest head is queried by go-legs via head-publisher.
		//
		//   2. The default selector is used where traversal stops at the
		//      latest known head.
		c, err := ing.sub.Sync(ctx, peerID, cid.Undef, nil, peerAddr)
		if err != nil {
			log.Errorw("Failed to sync with provider", "err", err, "provider", peerID)
			return
		}
		// Do not persist the latest sync here, because that is done in
		// after we've processed the ad.

		// If latest head had already finished syncing, then do not wait for syncDone since it will never happen.
		if latest == c {
			log.Infow("Latest advertisement already processed", "adCid", c)
			out <- c
			return
		}

		log.Debugw("Syncing advertisements up to latest", "adCid", c)
		for {
			select {
			case adCid := <-syncDone:
				log.Debugw("Synced advertisement", "adCid", adCid)
				if adCid == c {
					out <- c
					ing.signalMetricsUpdate()
					return
				}
			case <-ctx.Done():
				log.Warnw("Sync cancelled", "err", ctx.Err())
				return
			case <-ing.closePendingSyncs:
				log.Warnw("Sync cancelled because of close")
				return
			}
		}
	}()
	return out, nil
}

func (ing *Ingester) markAdProcessed(publisher peer.ID, adCid cid.Cid) error {
	log.Debugw("Persisted latest sync", "peer", publisher, "cid", adCid)
	err := ing.ds.Put(context.Background(), datastore.NewKey(adProcessedPrefix+adCid.String()), []byte{1})
	if err != nil {
		return err
	}
	return ing.ds.Put(context.Background(), datastore.NewKey(syncPrefix+publisher.String()), adCid.Bytes())
}

// distributeEvents reads a SyncFinished, sent by a peer handler, and copies
// the even to all channels in outEventsChans.  This delivers the SyncFinished
// to all onAdProcessed channel readers.
func (ing *Ingester) distributeEvents() {
	for event := range ing.inEvents {
		// Send update to all change notification channels.
		ing.outEventsMutex.Lock()
		outEventsChans, ok := ing.outEventsChans[event.PeerID]
		if ok {
			for _, ch := range outEventsChans {
				ch <- event.Cid
			}
		}
		ing.outEventsMutex.Unlock()
	}
}

// onAdProcessed creates a channel that receives notification when an
// advertisement and all of its content entries have finished syncing.
//
// Doing a manual sync will not always cause a notification if the requested
// advertisement has previously been processed.
//
// Calling the returned cancel function removes the notification channel from
// the list of channels to be notified on changes, and closes the channel to
// allow any reading goroutines to stop waiting on the channel.
func (ing *Ingester) onAdProcessed(peerID peer.ID) (<-chan cid.Cid, context.CancelFunc) {
	// Channel is buffered to prevent distribute() from blocking if a reader is
	// not reading the channel immediately.
	ch := make(chan cid.Cid, 1)
	ing.outEventsMutex.Lock()
	defer ing.outEventsMutex.Unlock()

	var outEventsChans []chan cid.Cid
	if ing.outEventsChans == nil {
		ing.outEventsChans = make(map[peer.ID][]chan cid.Cid)
	} else {
		outEventsChans = ing.outEventsChans[peerID]
	}
	ing.outEventsChans[peerID] = append(outEventsChans, ch)

	cncl := func() {
		ing.outEventsMutex.Lock()
		defer ing.outEventsMutex.Unlock()
		outEventsChans, ok := ing.outEventsChans[peerID]
		if !ok {
			return
		}

		for i, ca := range outEventsChans {
			if ca == ch {
				if len(outEventsChans) == 1 {
					if len(ing.outEventsChans) == 1 {
						ing.outEventsChans = nil
					} else {
						delete(ing.outEventsChans, peerID)
					}
				} else {
					outEventsChans[i] = outEventsChans[len(outEventsChans)-1]
					outEventsChans[len(outEventsChans)-1] = nil
					outEventsChans = outEventsChans[:len(outEventsChans)-1]
					close(ch)
					ing.outEventsChans[peerID] = outEventsChans
				}
				break
			}
		}
	}
	return ch, cncl
}

// signalMetricsUpdate signals that metrics should be updated.
func (ing *Ingester) signalMetricsUpdate() {
	select {
	case ing.sigUpdate <- struct{}{}:
	default:
		// Already signaled
	}
}

// metricsUpdate periodically updates metrics.  This goroutine exits when the
// sigUpdate channel is closed, when Close is called.
func (ing *Ingester) metricsUpdater() {
	hasUpdate := true
	t := time.NewTimer(time.Minute)

	for {
		select {
		case _, ok := <-ing.sigUpdate:
			if !ok {
				return
			}
			hasUpdate = true
		case <-t.C:
			if hasUpdate {
				// Update value store size metric after sync.
				size, err := ing.indexer.Size()
				if err != nil {
					log.Errorf("Error getting indexer value store size: %w", err)
					return
				}
				stats.Record(context.Background(), coremetrics.StoreSize.M(size))
				hasUpdate = false
			}
			t.Reset(time.Minute)
		}
	}
}

// restoreLatestSync reads the latest sync for each previously synced provider,
// from the datastore, and sets this in the Subscriber.
func (ing *Ingester) restoreLatestSync() error {
	// Load all pins from the datastore.
	q := query.Query{
		Prefix: syncPrefix,
	}
	results, err := ing.ds.Query(context.Background(), q)
	if err != nil {
		return err
	}
	defer results.Close()

	var count int
	for r := range results.Next() {
		if r.Error != nil {
			return fmt.Errorf("cannot read latest syncs: %w", r.Error)
		}
		ent := r.Entry
		_, lastCid, err := cid.CidFromBytes(ent.Value)
		if err != nil {
			log.Errorw("Failed to decode latest sync CID", "err", err)
			continue
		}
		if lastCid == cid.Undef {
			continue
		}
		peerID, err := peer.Decode(strings.TrimPrefix(ent.Key, syncPrefix))
		if err != nil {
			log.Errorw("Failed to decode peer ID of latest sync", "err", err)
			continue
		}

		err = ing.sub.SetLatestSync(peerID, lastCid)
		if err != nil {
			log.Errorw("Failed to set latest sync", "err", err, "peer", peerID)
			continue
		}
		log.Debugw("Set latest sync", "provider", peerID, "cid", lastCid)
		count++
	}
	log.Infow("Loaded latest sync for providers", "count", count)
	return nil
}

// Get the latest CID synced for the peer.
func (ing *Ingester) GetLatestSync(peerID peer.ID) (cid.Cid, error) {
	b, err := ing.ds.Get(context.Background(), datastore.NewKey(syncPrefix+peerID.String()))
	if err != nil {
		if err == datastore.ErrNotFound {
			return cid.Undef, nil
		}
		return cid.Undef, err
	}
	_, c, err := cid.CidFromBytes(b)
	return c, err
}

// removeEntryAdMappings removes all existing temporary entry cid to ad cid
// mappings.  If the indexer terminated unexpectedly during a sync operation,
// then these map be left over and should be cleaned up on restart.
func removeEntryAdMappings(ctx context.Context, ds datastore.Batching) error {
	q := query.Query{
		Prefix: admapPrefix,
	}
	results, err := ds.Query(ctx, q)
	if err != nil {
		return err
	}
	defer results.Close()

	var deletes []string
	for r := range results.Next() {
		if r.Error != nil {
			return err
		}
		ent := r.Entry
		deletes = append(deletes, ent.Key)
	}

	if len(deletes) != 0 {
		b, err := ds.Batch(ctx)
		if err != nil {
			return err
		}
		for i := range deletes {
			err = b.Delete(ctx, datastore.NewKey(deletes[i]))
			if err != nil {
				return err
			}
		}
		err = b.Commit(ctx)
		if err != nil {
			return err
		}

		log.Warnw("Cleaned up old temporary entry to ad mappings", "count", len(deletes))
	}
	return nil
}

type failedCid struct {
	c   cid.Cid
	err error
}

type toWorkerMsg struct {
	cids      []cid.Cid
	publisher peer.ID
	provider  peer.ID
}

func (ing *Ingester) loopIngester(workerPoolSize int) (failedCids []failedCid) {
	// startup the worker pool
	for i := 0; i < workerPoolSize; i++ {
		ing.waitForWorkers.Add(1)
		go func() {
			defer ing.waitForWorkers.Done()
			ing.ingestWorker()
		}()
	}

	for {
		fmt.Println("Start loop")
		select {
		case <-ing.closeWorkers:
			fmt.Println("CLOSING loop")
			return
		default:
			fmt.Println("Running step")
			ing.runIngestStep()
			fmt.Println("done Running step")
		}
	}
}

type cidsWithPublisher struct {
	cids []cid.Cid
	pub  peer.ID
}

func (ing *Ingester) runIngestStep() (failedCids []failedCid) {
	fmt.Println("running ingest step")
	defer fmt.Println("done running ingest step")
	syncFinishedEvent := <-ing.toStaging

	// 1. Group the incoming CIDs by provider.
	cidsGroupedByProvider := map[peer.ID][]cid.Cid{}
	for _, c := range syncFinishedEvent.SyncedCids {
		// Group the CIDs by the provider. Most of the time a publisher will only
		// publish Ads for one provider, but it's possible that an ad chain can include multiple providers.
		ad, err := ing.loadAd(c, true)
		if err != nil {
			log.Errorf("Failed to load ad CID: %s skipping", err)
			failedCids = append(failedCids, failedCid{c, err})
			continue
		}

		providerID, err := providerFromAd(ad)
		if err != nil {
			log.Errorf("Failed to load ad CID: %s skipping", err)
			failedCids = append(failedCids, failedCid{c, err})
			continue
		}

		cidsGroupedByProvider[providerID] = append(cidsGroupedByProvider[providerID], c)
	}

	// 2. Consolidate the new information into our staging area
	for p, cids := range cidsGroupedByProvider {
		ing.stagingProviderAds[p] = append(ing.stagingProviderAds[p], cidsWithPublisher{cids, syncFinishedEvent.PeerID})
	}

	// 3. For each group check if there is a running worker for that group. If not, put that stack in the toWorker chan to be processed
	// Put the msgs we are going to send in the array so that we get out of this lock zone.
	var toSend []toWorkerMsg
	ing.providersBeingProcessedMu.Lock()
	for p, cidsAndPub := range ing.stagingProviderAds {
		if len(cidsAndPub) == 0 {
			continue
		} else if beingProcessed := ing.providersBeingProcessed[p]; beingProcessed {
			// Skip over any peers that are currently being processed.
			continue
		} else {
			ing.providersBeingProcessed[p] = true
			toSend = append(toSend, toWorkerMsg{
				cids:      cidsAndPub[0].cids,
				publisher: cidsAndPub[0].pub,
				provider:  p,
			})
			ing.stagingProviderAds[p] = cidsAndPub[1:]
		}
	}
	ing.providersBeingProcessedMu.Unlock()

	for i, m := range toSend {
		fmt.Println("Send m", i+1, "of", len(toSend))
		ing.toWorkers <- m
	}

	return nil
}

func (ing *Ingester) ingestWorker() {
	for {
		select {
		case <-ing.closeWorkers:
			fmt.Println("CLOSING worker")
			return
		case msg := <-ing.toWorkers:
			fmt.Println("Worker running message:", msg.cids[0].String())
			for i := len(msg.cids) - 1; i >= 0; i-- {
				if ing.ingestWorkerLogic(msg.provider, msg.publisher, msg.cids[i]) {
					break
				}
			}
			fmt.Println("Done Worker running message:", msg)
		}
	}
}

func (ing *Ingester) ingestWorkerLogic(provider peer.ID, publisher peer.ID, c cid.Cid) (earlyBreak bool) {
	defer func() {
		ing.providersBeingProcessedMu.Lock()
		ing.providersBeingProcessed[provider] = false
		ing.providersBeingProcessedMu.Unlock()
	}()
	v, err := ing.ds.Get(context.Background(), datastore.NewKey(adProcessedPrefix+c.String()))
	if err == nil && v[0] == byte(1) {
		// We've process this ads already, so we know we've processed all earlier ads too.
		return true
	}
	ad, err := ing.loadAd(c, true)
	if err != nil {
		log.Errorf("Failed to load ad: %v", err)
	}
	fmt.Println("running sync", c)
	ing.syncAdEntries(publisher, ad, c)
	fmt.Println("done running sync")

	return false
}
