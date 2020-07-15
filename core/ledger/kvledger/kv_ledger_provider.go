/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package kvledger

import (
	"bytes"
	"fmt"
	"os"
	"path"

	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric-protos-go/common"
	"github.com/hyperledger/fabric/common/ledger/blkstorage"
	"github.com/hyperledger/fabric/common/ledger/dataformat"
	"github.com/hyperledger/fabric/common/ledger/util/leveldbhelper"
	"github.com/hyperledger/fabric/core/ledger"
	"github.com/hyperledger/fabric/core/ledger/confighistory"
	"github.com/hyperledger/fabric/core/ledger/kvledger/bookkeeping"
	"github.com/hyperledger/fabric/core/ledger/kvledger/history"
	"github.com/hyperledger/fabric/core/ledger/kvledger/msgs"
	"github.com/hyperledger/fabric/core/ledger/kvledger/txmgmt/privacyenabledstate"
	"github.com/hyperledger/fabric/core/ledger/pvtdatastorage"
	xcollstoreapi "github.com/hyperledger/fabric/extensions/collections/api/store"
	xledgerapi "github.com/hyperledger/fabric/extensions/ledger/api"
	"github.com/hyperledger/fabric/extensions/roles"
	xstorageapi "github.com/hyperledger/fabric/extensions/storage/api"
	xblkstorage "github.com/hyperledger/fabric/extensions/storage/blkstorage"
	xidstore "github.com/hyperledger/fabric/extensions/storage/idstore"
	xpvtdatastorage "github.com/hyperledger/fabric/extensions/storage/pvtdatastorage"
	"github.com/hyperledger/fabric/internal/fileutil"
	"github.com/hyperledger/fabric/protoutil"
	"github.com/pkg/errors"
	"github.com/syndtr/goleveldb/leveldb"
)

var (
	// ErrLedgerIDExists is thrown by a CreateLedger call if a ledger with the given id already exists
	ErrLedgerIDExists = errors.New("LedgerID already exists")
	// ErrNonExistingLedgerID is thrown by an OpenLedger call if a ledger with the given id does not exist
	ErrNonExistingLedgerID = errors.New("LedgerID does not exist")
	// ErrLedgerNotOpened is thrown by a CloseLedger call if a ledger with the given id has not been opened
	ErrLedgerNotOpened = errors.New("ledger is not opened yet")
	// ErrInactiveLedger is thrown by an OpenLedger call if a ledger with the given id is not active
	ErrInactiveLedger = errors.New("Ledger is not active")

	underConstructionLedgerKey = []byte("underConstructionLedgerKey")
	// ledgerKeyPrefix is the prefix for each ledger key in idStore db
	ledgerKeyPrefix = []byte{'l'}
	// ledgerKeyStop is the end key when querying idStore db by ledger key
	ledgerKeyStop = []byte{'l' + 1}
	// metadataKeyPrefix is the prefix for each metadata key in idStore db
	metadataKeyPrefix = []byte{'s'}
	// metadataKeyStop is the end key when querying idStore db by metadata key
	metadataKeyStop = []byte{'s' + 1}

	// formatKey
	formatKey = []byte("f")

	attrsToIndex = []blkstorage.IndexableAttr{
		blkstorage.IndexableAttrBlockHash,
		blkstorage.IndexableAttrBlockNum,
		blkstorage.IndexableAttrTxID,
		blkstorage.IndexableAttrBlockNumTranNum,
	}
)

const maxBlockFileSize = 64 * 1024 * 1024

// Provider implements interface ledger.PeerLedgerProvider
type Provider struct {
	idStore              xstorageapi.IDStore
	blkStoreProvider     xledgerapi.BlockStoreProvider
	pvtdataStoreProvider xstorageapi.PrivateDataProvider
	dbProvider           *privacyenabledstate.DBProvider
	historydbProvider    *history.DBProvider
	configHistoryMgr     *confighistory.Mgr
	stateListeners       []ledger.StateListener
	bookkeepingProvider  bookkeeping.Provider
	initializer          *ledger.Initializer
	collElgNotifier      *collElgNotifier
	stats                *stats
	fileLock             *leveldbhelper.FileLock

	collDataProvider xcollstoreapi.Provider
}

// NewProvider instantiates a new Provider.
// This is not thread-safe and assumed to be synchronized by the caller
func NewProvider(initializer *ledger.Initializer) (pr *Provider, e error) {
	p := &Provider{
		initializer: initializer,
	}

	defer func() {
		if e != nil {
			p.Close()
			if errFormatMismatch, ok := e.(*dataformat.ErrFormatMismatch); ok {
				if errFormatMismatch.Format == dataformat.PreviousFormat && errFormatMismatch.ExpectedFormat == dataformat.CurrentFormat {
					logger.Errorf("Please execute the 'peer node upgrade-dbs' command to upgrade the database format: %s", errFormatMismatch)
				} else {
					logger.Errorf("Please check the Fabric version matches the ledger data format: %s", errFormatMismatch)
				}
			}
		}
	}()

	fileLockPath := fileLockPath(initializer.Config.RootFSPath)
	fileLock := leveldbhelper.NewFileLock(fileLockPath)
	if err := fileLock.Lock(); err != nil {
		return nil, errors.Wrap(err, "as another peer node command is executing,"+
			" wait for that command to complete its execution or terminate it before retrying")
	}

	p.fileLock = fileLock

	if err := p.initLedgerIDInventory(); err != nil {
		return nil, err
	}
	if err := p.initBlockStoreProvider(); err != nil {
		return nil, err
	}
	if err := p.initPvtDataStoreProvider(); err != nil {
		return nil, err
	}
	if err := p.initHistoryDBProvider(); err != nil {
		return nil, err
	}
	if err := p.initConfigHistoryManager(); err != nil {
		return nil, err
	}
	p.initCollElgNotifier()
	p.initStateListeners()

	// State store must be initialized before ID store until
	// ID store in fabric-peer-ext supports format versioning
	if err := p.initStateDBProvider(); err != nil {
		return nil, err
	}

	p.initLedgerStatistics()
	p.recoverUnderConstructionLedger()
	if err := p.initSnapshotDir(); err != nil {
		return nil, err
	}

	if roles.IsCommitter() {
		p.recoverUnderConstructionLedger()
	}

	p.collDataProvider = initializer.CollDataProvider

	return p, nil
}

func (p *Provider) initLedgerIDInventory() error {
	idStore, err := xidstore.OpenIDStore(LedgerProviderPath(p.initializer.Config.RootFSPath), p.initializer.Config,
		func(path string, _ *ledger.Config) (xstorageapi.IDStore, error) {
			return openIDStore(path)
		},
	)
	if err != nil {
		return err
	}
	p.idStore = idStore
	return nil
}

func (p *Provider) initBlockStoreProvider() error {
	indexConfig := &blkstorage.IndexConfig{AttrsToIndex: attrsToIndex}
	blkStoreProvider, err := xblkstorage.NewProvider(
		blkstorage.NewConf(
			BlockStorePath(p.initializer.Config.RootFSPath),
			maxBlockFileSize,
		),
		indexConfig,
		p.initializer.Config,
		p.initializer.MetricsProvider,
	)
	if err != nil {
		return err
	}
	p.blkStoreProvider = blkStoreProvider
	return nil
}

func (p *Provider) initPvtDataStoreProvider() error {
	privateDataConfig := &pvtdatastorage.PrivateDataConfig{
		PrivateDataConfig: p.initializer.Config.PrivateDataConfig,
		StorePath:         PvtDataStorePath(p.initializer.Config.RootFSPath),
	}
	pvtdataStoreProvider, err := xpvtdatastorage.NewProvider(privateDataConfig, p.initializer.Config)
	if err != nil {
		return err
	}
	p.pvtdataStoreProvider = pvtdataStoreProvider
	return nil
}

func (p *Provider) initHistoryDBProvider() error {
	if !p.initializer.Config.HistoryDBConfig.Enabled {
		return nil
	}
	// Initialize the history database (index for history of values by key)
	historydbProvider, err := history.NewDBProvider(
		HistoryDBPath(p.initializer.Config.RootFSPath),
	)
	if err != nil {
		return err
	}
	p.historydbProvider = historydbProvider
	return nil
}

func (p *Provider) initConfigHistoryManager() error {
	var err error
	configHistoryMgr, err := confighistory.NewMgr(
		ConfigHistoryDBPath(p.initializer.Config.RootFSPath),
		p.initializer.DeployedChaincodeInfoProvider,
	)
	if err != nil {
		return err
	}
	p.configHistoryMgr = configHistoryMgr
	return nil
}

func (p *Provider) initCollElgNotifier() {
	collElgNotifier := &collElgNotifier{
		p.initializer.DeployedChaincodeInfoProvider,
		p.initializer.MembershipInfoProvider,
		make(map[string]collElgListener),
	}
	p.collElgNotifier = collElgNotifier
}

func (p *Provider) initStateListeners() {
	stateListeners := p.initializer.StateListeners
	stateListeners = append(stateListeners, p.collElgNotifier)
	stateListeners = append(stateListeners, p.configHistoryMgr)
	p.stateListeners = stateListeners
}

func (p *Provider) initStateDBProvider() error {
	var err error
	p.bookkeepingProvider, err = bookkeeping.NewProvider(
		BookkeeperDBPath(p.initializer.Config.RootFSPath),
	)
	if err != nil {
		return err
	}
	stateDB := &privacyenabledstate.StateDBConfig{
		StateDBConfig: p.initializer.Config.StateDBConfig,
		LevelDBPath:   StateDBPath(p.initializer.Config.RootFSPath),
	}
	sysNamespaces := p.initializer.DeployedChaincodeInfoProvider.Namespaces()
	p.dbProvider, err = privacyenabledstate.NewDBProvider(
		p.bookkeepingProvider,
		p.initializer.MetricsProvider,
		p.initializer.HealthCheckRegistry,
		stateDB,
		sysNamespaces,
	)
	return err
}

func (p *Provider) initLedgerStatistics() {
	p.stats = newStats(p.initializer.MetricsProvider)
}

func (p *Provider) initSnapshotDir() error {
	snapshotsRootDir := p.initializer.Config.SnapshotsConfig.RootDir
	if !path.IsAbs(snapshotsRootDir) {
		return errors.Errorf("invalid path: %s. The path for the snapshot dir is expected to be an absolute path", snapshotsRootDir)
	}

	inProgressSnapshotsPath := InProgressSnapshotsPath(snapshotsRootDir)
	completedSnapshotsPath := CompletedSnapshotsPath(snapshotsRootDir)

	if err := os.RemoveAll(inProgressSnapshotsPath); err != nil {
		return errors.Wrapf(err, "error while deleting the dir: %s", inProgressSnapshotsPath)
	}
	if err := os.MkdirAll(inProgressSnapshotsPath, 0755); err != nil {
		return errors.Wrapf(err, "error while creating the dir: %s", inProgressSnapshotsPath)
	}
	if err := os.MkdirAll(completedSnapshotsPath, 0755); err != nil {
		return errors.Wrapf(err, "error while creating the dir: %s", completedSnapshotsPath)
	}
	return fileutil.SyncDir(snapshotsRootDir)
}

// Create implements the corresponding method from interface ledger.PeerLedgerProvider
// This functions sets a under construction flag before doing any thing related to ledger creation and
// upon a successful ledger creation with the committed genesis block, removes the flag and add entry into
// created ledgers list (atomically). If a crash happens in between, the 'recoverUnderConstructionLedger'
// function is invoked before declaring the provider to be usable
func (p *Provider) Create(genesisBlock *common.Block) (ledger.PeerLedger, error) {
	ledgerID, err := protoutil.GetChannelIDFromBlock(genesisBlock)
	if err != nil {
		return nil, err
	}
	exists, err := p.idStore.LedgerIDExists(ledgerID)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrLedgerIDExists
	}
	if err = p.idStore.SetUnderConstructionFlag(ledgerID); err != nil {
		return nil, err
	}
	lgr, err := p.open(ledgerID)
	if err != nil {
		logger.Errorf("Error opening a new empty ledger. Unsetting under construction flag. Error: %+v", err)
		panicOnErr(p.runCleanup(ledgerID), "Error running cleanup for ledger id [%s]", ledgerID)
		panicOnErr(p.idStore.UnsetUnderConstructionFlag(), "Error while unsetting under construction flag")
		return nil, err
	}
	if err := lgr.CommitLegacy(&ledger.BlockAndPvtData{Block: genesisBlock}, &ledger.CommitOptions{}); err != nil {
		lgr.Close()
		return nil, err
	}
	panicOnErr(p.idStore.CreateLedgerID(ledgerID, genesisBlock), "Error while marking ledger as created")
	return lgr, nil
}

// Open implements the corresponding method from interface ledger.PeerLedgerProvider
func (p *Provider) Open(ledgerID string) (ledger.PeerLedger, error) {
	logger.Debugf("Open() opening kvledger: %s", ledgerID)
	// Check the ID store to ensure that the chainId/ledgerId exists
	active, exists, err := p.idStore.LedgerIDActive(ledgerID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrNonExistingLedgerID
	}
	if !active {
		return nil, ErrInactiveLedger
	}
	return p.open(ledgerID)
}

func (p *Provider) open(ledgerID string) (ledger.PeerLedger, error) {
	// Get the block store for a chain/ledger
	blockStore, err := p.blkStoreProvider.Open(ledgerID)
	if err != nil {
		return nil, err
	}

	pvtdataStore, err := p.pvtdataStoreProvider.OpenStore(ledgerID)
	if err != nil {
		return nil, err
	}

	p.collElgNotifier.registerListener(ledgerID, pvtdataStore)

	// Get the versioned database (state database) for a chain/ledger
	channelInfoProvider := &channelInfoProvider{ledgerID, blockStore, p.collElgNotifier.deployedChaincodeInfoProvider}
	db, err := p.dbProvider.GetDBHandle(ledgerID, channelInfoProvider)
	if err != nil {
		return nil, err
	}

	// Get the history database (index for history of values by key) for a chain/ledger
	var historyDB *history.DB
	if p.historydbProvider != nil {
		historyDB, err = p.historydbProvider.GetDBHandle(ledgerID)
		if err != nil {
			return nil, err
		}
	}

	initializer := &lgrInitializer{
		ledgerID:                 ledgerID,
		blockStore:               blockStore,
		pvtdataStore:             pvtdataStore,
		stateDB:                  db,
		historyDB:                historyDB,
		configHistoryMgr:         p.configHistoryMgr,
		stateListeners:           p.stateListeners,
		bookkeeperProvider:       p.bookkeepingProvider,
		ccInfoProvider:           p.initializer.DeployedChaincodeInfoProvider,
		ccLifecycleEventProvider: p.initializer.ChaincodeLifecycleEventProvider,
		stats:                    p.stats.ledgerStats(ledgerID),
		customTxProcessors:       p.initializer.CustomTxProcessors,
		hashProvider:             p.initializer.HashProvider,
		snapshotsConfig:          p.initializer.Config.SnapshotsConfig,
		collDataProvider:         p.initializer.CollDataProvider,
	}

	l, err := newKVLedger(initializer)
	if err != nil {
		return nil, err
	}
	return l, nil
}

// Exists implements the corresponding method from interface ledger.PeerLedgerProvider
func (p *Provider) Exists(ledgerID string) (bool, error) {
	return p.idStore.LedgerIDExists(ledgerID)
}

// List implements the corresponding method from interface ledger.PeerLedgerProvider
func (p *Provider) List() ([]string, error) {
	return p.idStore.GetActiveLedgerIDs()
}

// Close implements the corresponding method from interface ledger.PeerLedgerProvider
func (p *Provider) Close() {
	if p.idStore != nil {
		p.idStore.Close()
	}
	if p.blkStoreProvider != nil {
		p.blkStoreProvider.Close()
	}
	if p.pvtdataStoreProvider != nil {
		p.pvtdataStoreProvider.Close()
	}
	if p.dbProvider != nil {
		p.dbProvider.Close()
	}
	if p.bookkeepingProvider != nil {
		p.bookkeepingProvider.Close()
	}
	if p.configHistoryMgr != nil {
		p.configHistoryMgr.Close()
	}
	if p.historydbProvider != nil {
		p.historydbProvider.Close()
	}
	if p.fileLock != nil {
		p.fileLock.Unlock()
	}
}

// recoverUnderConstructionLedger checks whether the under construction flag is set - this would be the case
// if a crash had happened during creation of ledger and the ledger creation could have been left in intermediate
// state. Recovery checks if the ledger was created and the genesis block was committed successfully then it completes
// the last step of adding the ledger id to the list of created ledgers. Else, it clears the under construction flag
func (p *Provider) recoverUnderConstructionLedger() {
	logger.Debugf("Recovering under construction ledger")
	ledgerID, err := p.idStore.GetUnderConstructionFlag()
	panicOnErr(err, "Error while checking whether the under construction flag is set")
	if ledgerID == "" {
		logger.Debugf("No under construction ledger found. Quitting recovery")
		return
	}
	logger.Infof("ledger [%s] found as under construction", ledgerID)
	ledger, err := p.open(ledgerID)
	panicOnErr(err, "Error while opening under construction ledger [%s]", ledgerID)
	bcInfo, err := ledger.GetBlockchainInfo()
	panicOnErr(err, "Error while getting blockchain info for the under construction ledger [%s]", ledgerID)
	ledger.Close()

	switch bcInfo.Height {
	case 0:
		logger.Infof("Genesis block was not committed. Hence, the peer ledger not created. unsetting the under construction flag")
		panicOnErr(p.runCleanup(ledgerID), "Error while running cleanup for ledger id [%s]", ledgerID)
		panicOnErr(p.idStore.UnsetUnderConstructionFlag(), "Error while unsetting under construction flag")
	case 1:
		logger.Infof("Genesis block was committed. Hence, marking the peer ledger as created")
		genesisBlock, err := ledger.GetBlockByNumber(0)
		panicOnErr(err, "Error while retrieving genesis block from blockchain for ledger [%s]", ledgerID)
		panicOnErr(p.idStore.CreateLedgerID(ledgerID, genesisBlock), "Error while adding ledgerID [%s] to created list", ledgerID)
	default:
		panic(errors.Errorf(
			"data inconsistency: under construction flag is set for ledger [%s] while the height of the blockchain is [%d]",
			ledgerID, bcInfo.Height))
	}
}

// runCleanup cleans up blockstorage, statedb, and historydb for what
// may have got created during in-complete ledger creation
func (p *Provider) runCleanup(ledgerID string) error {
	// TODO - though, not having this is harmless for kv ledger.
	// If we want, following could be done:
	// - blockstorage could remove empty folders
	// - couchdb backed statedb could delete the database if got created
	// - leveldb backed statedb and history db need not perform anything as it uses a single db shared across ledgers
	return nil
}

func panicOnErr(err error, mgsFormat string, args ...interface{}) {
	if err == nil {
		return
	}
	args = append(args, err)
	panic(fmt.Sprintf(mgsFormat+" Error: %s", args...))
}

//////////////////////////////////////////////////////////////////////
// Ledger id persistence related code
///////////////////////////////////////////////////////////////////////
type idStore struct {
	db     *leveldbhelper.DB
	dbPath string
}

func openIDStore(path string) (s *idStore, e error) {
	db := leveldbhelper.CreateDB(&leveldbhelper.Conf{DBPath: path})
	db.Open()
	defer func() {
		if e != nil {
			db.Close()
		}
	}()

	emptyDB, err := db.IsEmpty()
	if err != nil {
		return nil, err
	}

	expectedFormatBytes := []byte(dataformat.CurrentFormat)
	if emptyDB {
		// add format key to a new db
		err := db.Put(formatKey, expectedFormatBytes, true)
		if err != nil {
			return nil, err
		}
		return &idStore{db, path}, nil
	}

	// verify the format is current for an existing db
	format, err := db.Get(formatKey)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(format, expectedFormatBytes) {
		logger.Errorf("The db at path [%s] contains data in unexpected format. expected data format = [%s] (%#v), data format = [%s] (%#v).",
			path, dataformat.CurrentFormat, expectedFormatBytes, format, format)
		return nil, &dataformat.ErrFormatMismatch{
			ExpectedFormat: dataformat.CurrentFormat,
			Format:         string(format),
			DBInfo:         fmt.Sprintf("leveldb for channel-IDs at [%s]", path),
		}
	}
	return &idStore{db, path}, nil
}

// checkUpgradeEligibility checks if the format is eligible to upgrade.
// It returns true if the format is eligible to upgrade to the current format.
// It returns false if either the format is the current format or the db is empty.
// Otherwise, an ErrFormatMismatch is returned.
func (s *idStore) checkUpgradeEligibility() (bool, error) {
	emptydb, err := s.db.IsEmpty()
	if err != nil {
		return false, err
	}
	if emptydb {
		logger.Warnf("Ledger database %s is empty, nothing to upgrade", s.dbPath)
		return false, nil
	}
	format, err := s.db.Get(formatKey)
	if err != nil {
		return false, err
	}
	if bytes.Equal(format, []byte(dataformat.CurrentFormat)) {
		logger.Debugf("Ledger database %s has current data format, nothing to upgrade", s.dbPath)
		return false, nil
	}
	if !bytes.Equal(format, []byte(dataformat.PreviousFormat)) {
		err = &dataformat.ErrFormatMismatch{
			ExpectedFormat: dataformat.PreviousFormat,
			Format:         string(format),
			DBInfo:         fmt.Sprintf("leveldb for channel-IDs at [%s]", s.dbPath),
		}
		return false, err
	}
	return true, nil
}

// GetFormat returns the database format
func (s *idStore) GetFormat() ([]byte, error) {
	return s.db.Get(formatKey)
}

func (s *idStore) UpgradeFormat() error {
	eligible, err := s.checkUpgradeEligibility()
	if err != nil {
		return err
	}
	if !eligible {
		return nil
	}

	logger.Infof("Upgrading ledgerProvider database to the new format %s", dataformat.CurrentFormat)

	batch := &leveldb.Batch{}
	batch.Put(formatKey, []byte(dataformat.CurrentFormat))

	// add new metadata key for each ledger (channel)
	metadata, err := protoutil.Marshal(&msgs.LedgerMetadata{Status: msgs.Status_ACTIVE})
	if err != nil {
		logger.Errorf("Error marshalling ledger metadata: %s", err)
		return errors.Wrapf(err, "error marshalling ledger metadata")
	}
	itr := s.db.GetIterator(ledgerKeyPrefix, ledgerKeyStop)
	defer itr.Release()
	for itr.Error() == nil && itr.Next() {
		id := s.decodeLedgerID(itr.Key(), ledgerKeyPrefix)
		batch.Put(s.encodeLedgerKey(id, metadataKeyPrefix), metadata)
	}
	if err = itr.Error(); err != nil {
		logger.Errorf("Error while upgrading idStore format: %s", err)
		return errors.Wrapf(err, "error while upgrading idStore format")
	}

	return s.db.WriteBatch(batch, true)
}

func (s *idStore) SetUnderConstructionFlag(ledgerID string) error {
	return s.db.Put(underConstructionLedgerKey, []byte(ledgerID), true)
}

func (s *idStore) UnsetUnderConstructionFlag() error {
	return s.db.Delete(underConstructionLedgerKey, true)
}

func (s *idStore) GetUnderConstructionFlag() (string, error) {
	val, err := s.db.Get(underConstructionLedgerKey)
	if err != nil {
		return "", err
	}
	return string(val), nil
}

func (s *idStore) CreateLedgerID(ledgerID string, gb *common.Block) error {
	gbKey := s.encodeLedgerKey(ledgerID, ledgerKeyPrefix)
	metadataKey := s.encodeLedgerKey(ledgerID, metadataKeyPrefix)
	var val []byte
	var metadata []byte
	var err error
	if val, err = s.db.Get(gbKey); err != nil {
		return err
	}
	if val != nil {
		return ErrLedgerIDExists
	}
	if val, err = proto.Marshal(gb); err != nil {
		return err
	}
	if metadata, err = protoutil.Marshal(&msgs.LedgerMetadata{Status: msgs.Status_ACTIVE}); err != nil {
		return err
	}
	batch := &leveldb.Batch{}
	batch.Put(gbKey, val)
	batch.Put(metadataKey, metadata)
	batch.Delete(underConstructionLedgerKey)
	return s.db.WriteBatch(batch, true)
}

func (s *idStore) UpdateLedgerStatus(ledgerID string, newStatus msgs.Status) error {
	metadata, err := s.getLedgerMetadata(ledgerID)
	if err != nil {
		return err
	}
	if metadata == nil {
		logger.Errorf("LedgerID [%s] does not exist", ledgerID)
		return ErrNonExistingLedgerID
	}
	if metadata.Status == newStatus {
		logger.Infof("Ledger [%s] is already in [%s] status, nothing to do", ledgerID, newStatus)
		return nil
	}
	metadata.Status = newStatus
	metadataBytes, err := proto.Marshal(metadata)
	if err != nil {
		logger.Errorf("Error marshalling ledger metadata: %s", err)
		return errors.Wrapf(err, "error marshalling ledger metadata")
	}
	logger.Infof("Updating ledger [%s] status to [%s]", ledgerID, newStatus)
	key := s.encodeLedgerKey(ledgerID, metadataKeyPrefix)
	return s.db.Put(key, metadataBytes, true)
}

func (s *idStore) getLedgerMetadata(ledgerID string) (*msgs.LedgerMetadata, error) {
	val, err := s.db.Get(s.encodeLedgerKey(ledgerID, metadataKeyPrefix))
	if val == nil || err != nil {
		return nil, err
	}
	metadata := &msgs.LedgerMetadata{}
	if err := proto.Unmarshal(val, metadata); err != nil {
		logger.Errorf("Error unmarshalling ledger metadata: %s", err)
		return nil, errors.Wrapf(err, "error unmarshalling ledger metadata")
	}
	return metadata, nil
}

func (s *idStore) LedgerIDExists(ledgerID string) (bool, error) {
	key := s.encodeLedgerKey(ledgerID, ledgerKeyPrefix)
	val, err := s.db.Get(key)
	if err != nil {
		return false, err
	}
	return val != nil, nil
}

// LedgerIDActive returns if a ledger is active and existed
func (s *idStore) LedgerIDActive(ledgerID string) (bool, bool, error) {
	metadata, err := s.getLedgerMetadata(ledgerID)
	if metadata == nil || err != nil {
		return false, false, err
	}
	return metadata.Status == msgs.Status_ACTIVE, true, nil
}

func (s *idStore) GetActiveLedgerIDs() ([]string, error) {
	var ids []string
	itr := s.db.GetIterator(metadataKeyPrefix, metadataKeyStop)
	defer itr.Release()
	for itr.Error() == nil && itr.Next() {
		metadata := &msgs.LedgerMetadata{}
		if err := proto.Unmarshal(itr.Value(), metadata); err != nil {
			logger.Errorf("Error unmarshalling ledger metadata: %s", err)
			return nil, errors.Wrapf(err, "error unmarshalling ledger metadata")
		}
		if metadata.Status == msgs.Status_ACTIVE {
			id := s.decodeLedgerID(itr.Key(), metadataKeyPrefix)
			ids = append(ids, id)
		}
	}
	if err := itr.Error(); err != nil {
		logger.Errorf("Error getting ledger ids from idStore: %s", err)
		return nil, errors.Wrapf(err, "error getting ledger ids from idStore")
	}
	return ids, nil
}

// GetGenesisBlock returns the genesis block for the given ledger ID
func (s *idStore) GetGenesisBlock(ledgerID string) (*common.Block, error) {
	bytes, err := s.db.Get(s.encodeLedgerKey(ledgerID, ledgerKeyPrefix))
	if err != nil {
		return nil, err
	}

	b := &common.Block{}
	err = proto.Unmarshal(bytes, b)
	if err != nil {
		return nil, err
	}

	return b, nil
}

func (s *idStore) Close() {
	s.db.Close()
}

func (s *idStore) encodeLedgerKey(ledgerID string, prefix []byte) []byte {
	return append(prefix, []byte(ledgerID)...)
}

func (s *idStore) decodeLedgerID(key []byte, prefix []byte) string {
	return string(key[len(prefix):])
}
