/*
Copyright IBM Corp. 2016 All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package txvalidator

import (
	"context"
	"fmt"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric-protos-go/common"
	mspprotos "github.com/hyperledger/fabric-protos-go/msp"
	"github.com/hyperledger/fabric-protos-go/peer"
	"github.com/hyperledger/fabric/bccsp"
	"github.com/hyperledger/fabric/common/channelconfig"
	"github.com/hyperledger/fabric/common/configtx"
	commonerrors "github.com/hyperledger/fabric/common/errors"
	"github.com/hyperledger/fabric/common/flogging"
	"github.com/hyperledger/fabric/core/committer/txvalidator/plugin"
	"github.com/hyperledger/fabric/core/common/sysccprovider"
	"github.com/hyperledger/fabric/core/common/validation"
	"github.com/hyperledger/fabric/core/ledger"
	ledgerUtil "github.com/hyperledger/fabric/core/ledger/util"
	"github.com/hyperledger/fabric/msp"
	"github.com/hyperledger/fabric/protoutil"
	"github.com/pkg/errors"
)

// Semaphore provides to the validator means for synchronisation
type Semaphore interface {
	// Acquire implements semaphore-like acquire semantics
	Acquire(ctx context.Context) error

	// Release implements semaphore-like release semantics
	Release()
}

// ChannelResources provides access to channel artefacts or
// functions to interact with them
type ChannelResources interface {
	// Ledger returns the ledger associated with this validator
	Ledger() ledger.PeerLedger

	// MSPManager returns the MSP manager for this channel
	MSPManager() msp.MSPManager

	// Apply attempts to apply a configtx to become the new config
	Apply(configtx *common.ConfigEnvelope) error

	// GetMSPIDs returns the IDs for the application MSPs
	// that have been defined in the channel
	GetMSPIDs() []string

	// Capabilities defines the capabilities for the application portion of this channel
	Capabilities() channelconfig.ApplicationCapabilities
}

// private interface to decouple tx validator
// and vscc execution, in order to increase
// testability of TxValidator
type vsccValidator interface {
	VSCCValidateTx(seq int, payload *common.Payload, envBytes []byte, block *common.Block) (error, peer.TxValidationCode)
}

// implementation of Validator interface, keeps
// reference to the ledger to enable tx simulation
// and execution of vscc
type TxValidator struct {
	ChannelID        string
	Semaphore        Semaphore
	ChannelResources ChannelResources
	Vscc             vsccValidator
	CryptoProvider   bccsp.BCCSP
}

var logger = flogging.MustGetLogger("committer.txvalidator")

type blockValidationRequest struct {
	block *common.Block
	d     []byte
	tIdx  int
}

type blockValidationResult struct {
	tIdx                 int
	validationCode       peer.TxValidationCode
	txsChaincodeName     *sysccprovider.ChaincodeInstance
	txsUpgradedChaincode *sysccprovider.ChaincodeInstance
	err                  error
	txid                 string
}

// NewTxValidator creates new transactions validator
func NewTxValidator(channelID string, sem Semaphore, cr ChannelResources, pm plugin.Mapper, cryptoProvider bccsp.BCCSP) *TxValidator {
	// Encapsulates interface implementation
	pluginValidator := NewPluginValidator(pm, cr.Ledger(), &dynamicDeserializer{cr: cr}, &dynamicCapabilities{cr: cr})
	return &TxValidator{
		ChannelID:        channelID,
		Semaphore:        sem,
		ChannelResources: cr,
		Vscc:             newVSCCValidator(channelID, cr, pluginValidator),
		CryptoProvider:   cryptoProvider,
	}
}

func (v *TxValidator) chainExists(chain string) bool {
	// TODO: implement this function!
	return true
}

// Validate performs the validation of a block. The validation
// of each transaction in the block is performed in parallel.
// The approach is as follows: the committer thread starts the
// tx validation function in a goroutine (using a semaphore to cap
// the number of concurrent validating goroutines). The committer
// thread then reads results of validation (in orderer of completion
// of the goroutines) from the results channel. The goroutines
// perform the validation of the txs in the block and enqueue the
// validation result in the results channel. A few note-worthy facts:
// 1) to keep the approach simple, the committer thread enqueues
//    all transactions in the block and then moves on to reading the
//    results.
// 2) for parallel validation to work, it is important that the
//    validation function does not change the state of the system.
//    Otherwise the order in which validation is perform matters
//    and we have to resort to sequential validation (or some locking).
//    This is currently true, because the only function that affects
//    state is when a config transaction is received, but they are
//    guaranteed to be alone in the block. If/when this assumption
//    is violated, this code must be changed.
func (v *TxValidator) Validate(block *common.Block) error {
	var err error
	var errPos int

	startValidation := time.Now() // timer to log Validate block duration
	logger.Debugf("[%s] START Block Validation for block [%d]", v.ChannelID, block.Header.Number)

	// Initialize trans as not_validated here, then set invalidation reason code upon invalidation below
	txsfltr := ledgerUtil.NewTxValidationFlags(len(block.Data.Data))
	// txsChaincodeNames records all the invoked chaincodes by tx in a block
	txsChaincodeNames := make(map[int]*sysccprovider.ChaincodeInstance)
	// upgradedChaincodes records all the chaincodes that are upgraded in a block
	txsUpgradedChaincodes := make(map[int]*sysccprovider.ChaincodeInstance)
	// array of txids
	txidArray := make([]string, len(block.Data.Data))

	results := make(chan *blockValidationResult)
	go func() {
		for tIdx, d := range block.Data.Data {
			// ensure that we don't have too many concurrent validation workers
			v.Semaphore.Acquire(context.Background())

			go func(index int, data []byte) {
				defer v.Semaphore.Release()

				v.validateTx(&blockValidationRequest{
					d:     data,
					block: block,
					tIdx:  index,
				}, results)
			}(tIdx, d)
		}
	}()

	logger.Debugf("expecting %d block validation responses", len(block.Data.Data))

	// now we read responses in the order in which they come back
	for i := 0; i < len(block.Data.Data); i++ {
		res := <-results

		if res.err != nil {
			// if there is an error, we buffer its value, wait for
			// all workers to complete validation and then return
			// the error from the first tx in this block that returned an error
			logger.Debugf("got terminal error %s for idx %d", res.err, res.tIdx)

			if err == nil || res.tIdx < errPos {
				err = res.err
				errPos = res.tIdx
			}
		} else {
			// if there was no error, we set the txsfltr and we set the
			// txsChaincodeNames and txsUpgradedChaincodes maps
			logger.Debugf("got result for idx %d, code %d", res.tIdx, res.validationCode)

			txsfltr.SetFlag(res.tIdx, res.validationCode)

			if res.validationCode == peer.TxValidationCode_VALID {
				if res.txsChaincodeName != nil {
					txsChaincodeNames[res.tIdx] = res.txsChaincodeName
				}
				if res.txsUpgradedChaincode != nil {
					txsUpgradedChaincodes[res.tIdx] = res.txsUpgradedChaincode
				}
				txidArray[res.tIdx] = res.txid
			}
		}
	}

	// if we're here, all workers have completed the validation.
	// If there was an error we return the error from the first
	// tx in this block that returned an error
	if err != nil {
		return err
	}

	// if we operate with this capability, we mark invalid any transaction that has a txid
	// which is equal to that of a previous tx in this block
	if v.ChannelResources.Capabilities().ForbidDuplicateTXIdInBlock() {
		markTXIdDuplicates(txidArray, txsfltr)
	}

	// if we're here, all workers have completed validation and
	// no error was reported; we set the tx filter and return
	// success
	v.invalidTXsForUpgradeCC(txsChaincodeNames, txsUpgradedChaincodes, txsfltr)

	// make sure no transaction has skipped validation
	err = v.allValidated(txsfltr, block)
	if err != nil {
		return err
	}

	// Initialize metadata structure
	protoutil.InitBlockMetadata(block)

	block.Metadata.Metadata[common.BlockMetadataIndex_TRANSACTIONS_FILTER] = txsfltr

	elapsedValidation := time.Since(startValidation) / time.Millisecond // duration in ms
	logger.Infof("[%s] Validated block [%d] in %dms", v.ChannelID, block.Header.Number, elapsedValidation)

	return nil
}

// allValidated returns error if some of the validation flags have not been set
// during validation
func (v *TxValidator) allValidated(txsfltr ledgerUtil.TxValidationFlags, block *common.Block) error {
	for id, f := range txsfltr {
		if peer.TxValidationCode(f) == peer.TxValidationCode_NOT_VALIDATED {
			return errors.Errorf("transaction %d in block %d has skipped validation", id, block.Header.Number)
		}
	}

	return nil
}

func markTXIdDuplicates(txids []string, txsfltr ledgerUtil.TxValidationFlags) {
	txidMap := make(map[string]struct{})

	for id, txid := range txids {
		if txid == "" {
			continue
		}

		_, in := txidMap[txid]
		if in {
			logger.Error("Duplicate txid", txid, "found, skipping")
			txsfltr.SetFlag(id, peer.TxValidationCode_DUPLICATE_TXID)
		} else {
			txidMap[txid] = struct{}{}
		}
	}
}

func (v *TxValidator) validateTx(req *blockValidationRequest, results chan<- *blockValidationResult) {
	block := req.block
	d := req.d
	tIdx := req.tIdx
	txID := ""

	if d == nil {
		results <- &blockValidationResult{
			tIdx: tIdx,
		}
		return
	}

	if env, err := protoutil.GetEnvelopeFromBlock(d); err != nil {
		logger.Warningf("Error getting tx from block: %+v", err)
		results <- &blockValidationResult{
			tIdx:           tIdx,
			validationCode: peer.TxValidationCode_INVALID_OTHER_REASON,
		}
		return
	} else if env != nil {
		// validate the transaction: here we check that the transaction
		// is properly formed, properly signed and that the security
		// chain binding proposal to endorsements to tx holds. We do
		// NOT check the validity of endorsements, though. That's a
		// job for VSCC below
		logger.Debugf("[%s] validateTx starts for block %p env %p txn %d", v.ChannelID, block, env, tIdx)
		defer logger.Debugf("[%s] validateTx completes for block %p env %p txn %d", v.ChannelID, block, env, tIdx)
		var payload *common.Payload
		var err error
		var txResult peer.TxValidationCode
		var txsChaincodeName *sysccprovider.ChaincodeInstance
		var txsUpgradedChaincode *sysccprovider.ChaincodeInstance

		if payload, txResult = validation.ValidateTransaction(env, v.CryptoProvider); txResult != peer.TxValidationCode_VALID {
			logger.Errorf("Invalid transaction with index %d", tIdx)
			results <- &blockValidationResult{
				tIdx:           tIdx,
				validationCode: txResult,
			}
			return
		}

		chdr, err := protoutil.UnmarshalChannelHeader(payload.Header.ChannelHeader)
		if err != nil {
			logger.Warningf("Could not unmarshal channel header, err %s, skipping", err)
			results <- &blockValidationResult{
				tIdx:           tIdx,
				validationCode: peer.TxValidationCode_INVALID_OTHER_REASON,
			}
			return
		}

		channel := chdr.ChannelId
		logger.Debugf("Transaction is for channel %s", channel)

		if !v.chainExists(channel) {
			logger.Errorf("Dropping transaction for non-existent channel %s", channel)
			results <- &blockValidationResult{
				tIdx:           tIdx,
				validationCode: peer.TxValidationCode_TARGET_CHAIN_NOT_FOUND,
			}
			return
		}

		if common.HeaderType(chdr.Type) == common.HeaderType_ENDORSER_TRANSACTION {

			txID = chdr.TxId

			// Check duplicate transactions
			erroneousResultEntry := v.checkTxIdDupsLedger(tIdx, chdr, v.ChannelResources.Ledger())
			if erroneousResultEntry != nil {
				results <- erroneousResultEntry
				return
			}

			// Validate tx with vscc and policy
			logger.Debug("Validating transaction vscc tx validate")
			err, cde := v.Vscc.VSCCValidateTx(tIdx, payload, d, block)
			if err != nil {
				logger.Errorf("VSCCValidateTx for transaction txId = %s returned error: %s", txID, err)
				switch err.(type) {
				case *commonerrors.VSCCExecutionFailureError:
					results <- &blockValidationResult{
						tIdx: tIdx,
						err:  err,
					}
					return
				case *commonerrors.VSCCInfoLookupFailureError:
					results <- &blockValidationResult{
						tIdx: tIdx,
						err:  err,
					}
					return
				default:
					results <- &blockValidationResult{
						tIdx:           tIdx,
						validationCode: cde,
					}
					return
				}
			}

			invokeCC, upgradeCC, err := v.getTxCCInstance(payload)
			if err != nil {
				logger.Errorf("Get chaincode instance from transaction txId = %s returned error: %+v", txID, err)
				results <- &blockValidationResult{
					tIdx:           tIdx,
					validationCode: peer.TxValidationCode_INVALID_OTHER_REASON,
				}
				return
			}
			txsChaincodeName = invokeCC
			if upgradeCC != nil {
				logger.Infof("Find chaincode upgrade transaction for chaincode %s on channel %s with new version %s", upgradeCC.ChaincodeName, upgradeCC.ChannelID, upgradeCC.ChaincodeVersion)
				txsUpgradedChaincode = upgradeCC
			}
		} else if common.HeaderType(chdr.Type) == common.HeaderType_CONFIG {
			configEnvelope, err := configtx.UnmarshalConfigEnvelope(payload.Data)
			if err != nil {
				err = errors.WithMessage(err, "error unmarshalling config which passed initial validity checks")
				logger.Criticalf("%+v", err)
				results <- &blockValidationResult{
					tIdx: tIdx,
					err:  err,
				}
				return
			}

			if err := v.ChannelResources.Apply(configEnvelope); err != nil {
				err = errors.WithMessage(err, "error validating config which passed initial validity checks")
				logger.Criticalf("%+v", err)
				results <- &blockValidationResult{
					tIdx: tIdx,
					err:  err,
				}
				return
			}
			logger.Debugf("config transaction received for chain %s", channel)
		} else {
			logger.Warningf("Unknown transaction type [%s] in block number [%d] transaction index [%d]",
				common.HeaderType(chdr.Type), block.Header.Number, tIdx)
			results <- &blockValidationResult{
				tIdx:           tIdx,
				validationCode: peer.TxValidationCode_UNKNOWN_TX_TYPE,
			}
			return
		}

		if _, err := proto.Marshal(env); err != nil {
			logger.Warningf("Cannot marshal transaction: %s", err)
			results <- &blockValidationResult{
				tIdx:           tIdx,
				validationCode: peer.TxValidationCode_MARSHAL_TX_ERROR,
			}
			return
		}
		// Succeeded to pass down here, transaction is valid
		results <- &blockValidationResult{
			tIdx:                 tIdx,
			txsChaincodeName:     txsChaincodeName,
			txsUpgradedChaincode: txsUpgradedChaincode,
			validationCode:       peer.TxValidationCode_VALID,
			txid:                 txID,
		}
		return
	} else {
		logger.Warning("Nil tx from block")
		results <- &blockValidationResult{
			tIdx:           tIdx,
			validationCode: peer.TxValidationCode_NIL_ENVELOPE,
		}
		return
	}
}

// CheckTxIdDupsLedger returns a vlockValidationResult enhanced with the respective
// error codes if and only if there is transaction with the same transaction identifier
// in the ledger or no decision can be made for whether such transaction exists;
// the function returns nil if it has ensured that there is no such duplicate, such
// that its consumer can proceed with the transaction processing
func (v *TxValidator) checkTxIdDupsLedger(tIdx int, chdr *common.ChannelHeader, ldgr ledger.PeerLedger) *blockValidationResult {

	// Retrieve the transaction identifier of the input header
	txID := chdr.TxId

	// Look for a transaction with the same identifier inside the ledger
	_, err := ldgr.GetTransactionByID(txID)

	switch err.(type) {
	case nil:
		// invalid case, returned error is nil. It means that there is already a tx in the ledger with the same id
		logger.Error("Duplicate transaction found, ", txID, ", skipping")
		return &blockValidationResult{
			tIdx:           tIdx,
			validationCode: peer.TxValidationCode_DUPLICATE_TXID,
		}
	case ledger.NotFoundInIndexErr:
		// valid case, returned error is of type NotFoundInIndexErr.
		// It means that no tx with the same id is found in the ledger
		return nil
	default:
		// invalid case, returned error is not of type NotFoundInIndexErr.
		// It means that we could not verify whether a tx with the supplied id is in the ledger
		logger.Errorf("Ledger failure while attempting to detect duplicate status for txid %s: %s", txID, err)
		return &blockValidationResult{
			tIdx: tIdx,
			err:  err,
		}
	}
}

// generateCCKey generates a unique identifier for chaincode in specific channel
func (v *TxValidator) generateCCKey(ccName, chainID string) string {
	return fmt.Sprintf("%s/%s", ccName, chainID)
}

// invalidTXsForUpgradeCC invalid all txs that should be invalided because of chaincode upgrade txs
func (v *TxValidator) invalidTXsForUpgradeCC(txsChaincodeNames map[int]*sysccprovider.ChaincodeInstance, txsUpgradedChaincodes map[int]*sysccprovider.ChaincodeInstance, txsfltr ledgerUtil.TxValidationFlags) {
	if len(txsUpgradedChaincodes) == 0 {
		return
	}

	// Invalid former cc upgrade txs if there're two or more txs upgrade the same cc
	finalValidUpgradeTXs := make(map[string]int)
	upgradedChaincodes := make(map[string]*sysccprovider.ChaincodeInstance)
	for tIdx, cc := range txsUpgradedChaincodes {
		if cc == nil {
			continue
		}
		upgradedCCKey := v.generateCCKey(cc.ChaincodeName, cc.ChannelID)

		if finalIdx, exist := finalValidUpgradeTXs[upgradedCCKey]; !exist {
			finalValidUpgradeTXs[upgradedCCKey] = tIdx
			upgradedChaincodes[upgradedCCKey] = cc
		} else if finalIdx < tIdx {
			logger.Infof("Invalid transaction with index %d: chaincode was upgraded by latter tx", finalIdx)
			txsfltr.SetFlag(finalIdx, peer.TxValidationCode_CHAINCODE_VERSION_CONFLICT)

			// record latter cc upgrade tx info
			finalValidUpgradeTXs[upgradedCCKey] = tIdx
			upgradedChaincodes[upgradedCCKey] = cc
		} else {
			logger.Infof("Invalid transaction with index %d: chaincode was upgraded by latter tx", tIdx)
			txsfltr.SetFlag(tIdx, peer.TxValidationCode_CHAINCODE_VERSION_CONFLICT)
		}
	}

	// invalid txs which invoke the upgraded chaincodes
	for tIdx, cc := range txsChaincodeNames {
		if cc == nil {
			continue
		}
		ccKey := v.generateCCKey(cc.ChaincodeName, cc.ChannelID)
		if _, exist := upgradedChaincodes[ccKey]; exist {
			if txsfltr.IsValid(tIdx) {
				logger.Infof("Invalid transaction with index %d: chaincode was upgraded in the same block", tIdx)
				txsfltr.SetFlag(tIdx, peer.TxValidationCode_CHAINCODE_VERSION_CONFLICT)
			}
		}
	}
}

func (v *TxValidator) getTxCCInstance(payload *common.Payload) (invokeCCIns, upgradeCCIns *sysccprovider.ChaincodeInstance, err error) {
	// This is duplicated unpacking work, but make test easier.
	chdr, err := protoutil.UnmarshalChannelHeader(payload.Header.ChannelHeader)
	if err != nil {
		return nil, nil, err
	}

	// Channel ID
	channelID := chdr.ChannelId // it is guaranteed to be an existing channel by now

	// ChaincodeID
	hdrExt, err := protoutil.UnmarshalChaincodeHeaderExtension(chdr.Extension)
	if err != nil {
		return nil, nil, err
	}
	invokeCC := hdrExt.ChaincodeId
	invokeIns := &sysccprovider.ChaincodeInstance{ChannelID: channelID, ChaincodeName: invokeCC.Name, ChaincodeVersion: invokeCC.Version}

	// Transaction
	tx, err := protoutil.UnmarshalTransaction(payload.Data)
	if err != nil {
		logger.Errorf("GetTransaction failed: %+v", err)
		return invokeIns, nil, nil
	}

	// ChaincodeActionPayload
	cap, err := protoutil.UnmarshalChaincodeActionPayload(tx.Actions[0].Payload)
	if err != nil {
		logger.Errorf("GetChaincodeActionPayload failed: %+v", err)
		return invokeIns, nil, nil
	}

	// ChaincodeProposalPayload
	cpp, err := protoutil.UnmarshalChaincodeProposalPayload(cap.ChaincodeProposalPayload)
	if err != nil {
		logger.Errorf("GetChaincodeProposalPayload failed: %+v", err)
		return invokeIns, nil, nil
	}

	// ChaincodeInvocationSpec
	cis := &peer.ChaincodeInvocationSpec{}
	err = proto.Unmarshal(cpp.Input, cis)
	if err != nil {
		logger.Errorf("GetChaincodeInvokeSpec failed: %+v", err)
		return invokeIns, nil, nil
	}

	if invokeCC.Name == "lscc" {
		if string(cis.ChaincodeSpec.Input.Args[0]) == "upgrade" {
			upgradeIns, err := v.getUpgradeTxInstance(channelID, cis.ChaincodeSpec.Input.Args[2])
			if err != nil {
				return invokeIns, nil, nil
			}
			return invokeIns, upgradeIns, nil
		}
	}

	return invokeIns, nil, nil
}

func (v *TxValidator) getUpgradeTxInstance(channelID string, cdsBytes []byte) (*sysccprovider.ChaincodeInstance, error) {
	cds, err := protoutil.UnmarshalChaincodeDeploymentSpec(cdsBytes)
	if err != nil {
		return nil, err
	}

	if cds.ChaincodeSpec.Type.String() != "GOLANG" {
		return nil, errors.Errorf("unexpected chaincode type: %s", cds.ChaincodeSpec.Type.String())
	}

	return &sysccprovider.ChaincodeInstance{
		ChannelID:        channelID,
		ChaincodeName:    cds.ChaincodeSpec.ChaincodeId.Name,
		ChaincodeVersion: cds.ChaincodeSpec.ChaincodeId.Version,
	}, nil
}

type dynamicDeserializer struct {
	cr ChannelResources
}

func (ds *dynamicDeserializer) DeserializeIdentity(serializedIdentity []byte) (msp.Identity, error) {
	return ds.cr.MSPManager().DeserializeIdentity(serializedIdentity)
}

func (ds *dynamicDeserializer) IsWellFormed(identity *mspprotos.SerializedIdentity) error {
	return ds.cr.MSPManager().IsWellFormed(identity)
}

type dynamicCapabilities struct {
	cr ChannelResources
}

func (ds *dynamicCapabilities) ACLs() bool {
	return ds.cr.Capabilities().ACLs()
}

func (ds *dynamicCapabilities) CollectionUpgrade() bool {
	return ds.cr.Capabilities().CollectionUpgrade()
}

func (ds *dynamicCapabilities) StorePvtDataOfInvalidTx() bool {
	return ds.cr.Capabilities().StorePvtDataOfInvalidTx()
}

func (ds *dynamicCapabilities) ForbidDuplicateTXIdInBlock() bool {
	return ds.cr.Capabilities().ForbidDuplicateTXIdInBlock()
}

func (ds *dynamicCapabilities) KeyLevelEndorsement() bool {
	return ds.cr.Capabilities().KeyLevelEndorsement()
}

func (ds *dynamicCapabilities) MetadataLifecycle() bool {
	// This capability no longer exists and should not be referenced in validation anyway
	return false
}

func (ds *dynamicCapabilities) PrivateChannelData() bool {
	return ds.cr.Capabilities().PrivateChannelData()
}

func (ds *dynamicCapabilities) Supported() error {
	return ds.cr.Capabilities().Supported()
}

func (ds *dynamicCapabilities) V1_1Validation() bool {
	return ds.cr.Capabilities().V1_1Validation()
}

func (ds *dynamicCapabilities) V1_2Validation() bool {
	return ds.cr.Capabilities().V1_2Validation()
}

func (ds *dynamicCapabilities) V1_3Validation() bool {
	return ds.cr.Capabilities().V1_3Validation()
}

func (ds *dynamicCapabilities) V2_0Validation() bool {
	return ds.cr.Capabilities().V2_0Validation()
}
