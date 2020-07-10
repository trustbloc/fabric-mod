/*
Copyright IBM Corp. 2017 All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package follower

import (
	"sync"
	"time"

	"github.com/hyperledger/fabric-protos-go/common"
	"github.com/hyperledger/fabric/bccsp"
	"github.com/hyperledger/fabric/common/flogging"
	"github.com/hyperledger/fabric/orderer/common/cluster"
	"github.com/hyperledger/fabric/orderer/common/types"
	"github.com/hyperledger/fabric/orderer/consensus"
	"github.com/pkg/errors"
)

//TODO skeleton

//go:generate mockery -dir . -name Support -case underscore -output mocks

// Support defines the interfaces needed by the follower.Chain, out of the much wider consensus.ConsenterSupport.
type Support interface {
	consensus.ConsenterSupport // TODO prune what we don't need, keep only what we do need.
}

// Options contains all the configurations relevant to the chain.
type Options struct {
	Logger *flogging.FabricLogger
	Cert   []byte
}

// CreateBlockPuller is a function to create BlockPuller on demand.
// It is passed into chain initializer so that tests could mock this.
type CreateBlockPuller func() (ChannelPuller, error)

// CreateChain is a function that creates a new consensus.Chain for this channel, to replace the current follower.Chain
type CreateChain func()

// Chain implements a component that allows the orderer to follow a specific channel when is not a cluster member,
// that is, be a "follower" of the cluster. It also allows the orderer to perform "on-boarding" for channels for
// channels it is joining as a member, with a join-block.
//
// When an orderer is following a channel, it means that the current orderer is not a member of the consenters set
// of the channel, and is only pulling blocks from other orderers. In this mode, the follower is inspecting config
// blocks as they are pulled and if it discovers that it was introduced into the consenters set, it will trigger the
// creation of a regular etcdraft.Chain, that is, turn into a "member" of the cluster.
//
// A follower is also used to on-board a channel when joining as a member with a join-block that has number >0. In this
// mode the follower will pull blocks up until join-block.number, and then will trigger the creation of a regular
// etcdraft.Chain.
//
// The follower is started in one of two ways: 1) following an API Join request with a join-block that has
// block number >0, or 2) when the orderer was a cluster member (i.e. was running a etcdraft.Chain) and was removed
// from the consenters set.
//
// The follower is in status "onboarding" when it pulls blocks below the join-block number, or "active" when it
// pulls blocks equal or above the join-block number.
//
// The follower return clusterRelation "member" when the join-block indicates the orderer is in the consenters set,
// i.e. the follower is performing on-boarding for an etcdraft.Chain. Otherwise, the follower return clusterRelation
// "follower".
type Chain struct {
	mutex    sync.Mutex    // Protects the start/stop flags & channels, all the rest are immutable or accessed only by the go-routine.
	started  bool          // Start once.
	stopped  bool          // Stop once.
	stopChan chan struct{} // A 'closer' signals the go-routine to stop by closing this channel.
	doneChan chan struct{} // The go-routine signals the 'closer' that it is done by closing this channel.

	support Support
	options Options
	logger  *flogging.FabricLogger

	joinBlock  *common.Block // The join-block the follower was started with.
	lastConfig *common.Block // The last config block from the ledger. Accessed only by the go-routine.

	createBlockPullerFunc CreateBlockPuller // A method that creates a block puller on demand
	chainCreationCallback CreateChain       // A method that creates a new consensus.Chain for this channel, to replace the current follower.Chain

	cryptoProvider bccsp.BCCSP
	amIInChannel   cluster.SelfMembershipPredicate
}

// NewChain constructs a follower.Chain object.
func NewChain(
	support Support,
	joinBlock *common.Block,
	options Options,
	createBlockPullerFunc CreateBlockPuller, // A method that creates a block puller on demand
	chainCreationCallback func(),
	cryptoProvider bccsp.BCCSP,
	amIInChannel cluster.SelfMembershipPredicate,
) (*Chain, error) {
	chain := &Chain{
		stopChan:              make(chan (struct{})),
		doneChan:              make(chan (struct{})),
		support:               support,
		joinBlock:             joinBlock,
		options:               options,
		logger:                options.Logger.With("channel", support.ChannelID()),
		createBlockPullerFunc: createBlockPullerFunc,
		chainCreationCallback: chainCreationCallback,
		cryptoProvider:        cryptoProvider,
		amIInChannel:          amIInChannel,
	}

	if joinBlock == nil {
		chain.logger.Infof("Created with a nil join-block, ledger height: %d", support.Height())
	} else {
		if joinBlock.Header == nil {
			return nil, errors.New("block header is nil")
		}
		if joinBlock.Data == nil {
			return nil, errors.New("block data is nil")
		}

		chain.logger.Infof("Created with join-block number: %d, ledger height: %d", joinBlock.Header.Number, support.Height())
	}

	return chain, nil
}

func (c *Chain) Order(_ *common.Envelope, _ uint64) error {
	return errors.Errorf("orderer is a follower of channel %s", c.support.ChannelID())
}

func (c *Chain) Configure(_ *common.Envelope, _ uint64) error {
	return errors.Errorf("orderer is a follower of channel %s", c.support.ChannelID())
}

func (c *Chain) WaitReady() error {
	return errors.Errorf("orderer is a follower of channel %s", c.support.ChannelID())
}

func (*Chain) Errored() <-chan struct{} {
	closedChannel := make(chan struct{})
	close(closedChannel)
	return closedChannel
}

func (c *Chain) Start() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.started || c.stopped {
		c.logger.Debugf("Not starting because: started=%v, stopped=%v", c.started, c.stopped)
		return
	}

	c.started = true

	go c.run()

	c.logger.Info("Started")
}

func (c *Chain) Halt() {
	c.halt()

	select {
	case <-c.doneChan:
	}
}

func (c *Chain) halt() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.stopped {
		c.logger.Debug("Already stopped")
		return
	}
	c.stopped = true
	close(c.stopChan)
	c.logger.Info("Stopped")
}

// StatusReport returns the ClusterRelation & Status.
func (c *Chain) StatusReport() (types.ClusterRelation, types.Status) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	clusterRelation := types.ClusterRelationFollower
	if c.joinBlock != nil {
		if err := c.amIInChannel(c.joinBlock); err == nil {
			clusterRelation = types.ClusterRelationMember
		}
	}

	status := types.StatusActive
	if (c.joinBlock != nil) && (c.joinBlock.Header.Number >= c.support.Height()) {
		status = types.StatusOnBoarding
	}

	return clusterRelation, status
}

func (c *Chain) run() {
	c.logger.Debug("The follower.Chain puller goroutine is starting")
	pullRetryTicker := time.NewTicker(500 * time.Millisecond) // TODO move constant to options

	defer func() {
		pullRetryTicker.Stop()
		close(c.doneChan)
		c.logger.Debug("The follower.Chain puller goroutine is exiting")
	}()

	err := c.pull()

	for err != nil {
		c.logger.Debugf("Pull failed, going to try again in %v; error: %s", pullRetryTicker, err)

		select {
		case <-c.stopChan:
			c.logger.Debug("Received a stop signal")
			return

		case <-pullRetryTicker.C:
			err = c.pull()
		}
	}
}

// pull blocks from other orderers, until the join block, or until a config block the indicates the orderer has become
// a member pf the cluster. When the follower.Chain's job is done, this method halts, triggers the creation of a new
// consensus.Chain, and returns nil.
func (c *Chain) pull() error {
	if c.joinBlock != nil {
		err := c.pullUpToJoin()
		if err != nil {
			return errors.Wrap(err, "failed to pull up to join block")
		}

		// TODO remove the join-block from the file repo
	} else {
		err := c.pullAfterJoin()
		if err != nil {
			return errors.Wrap(err, "failed to pull after join block")
		}
	}

	// Trigger creation of a new chain. This will regenerate 'support' from the tip of the ledger, and will
	// start either a follower.Chain or an etcdraft.Chain, depending on whether the orderer is a follower or a
	// member of the cluster.
	c.halt()
	if c.chainCreationCallback != nil {
		c.logger.Info("Block pulling finished successfully, invoking chain creation callback")
		c.chainCreationCallback()
	}

	return nil
}

// pullUpToJoin pulls blocks up to the join-block height without inspecting membership on fetched config blocks.
// It checks whether the chain was stopped between blocks.
func (c *Chain) pullUpToJoin() error {
	//TODO
	return errors.New("not implemented yet: pull up to join block")
}

// pullAfterJoin pulls blocks continuously, inspecting the fetched config blocks for membership.
// It will exit with 'nil' if it detects a config block that indicates the orderer is a member of the cluster.
// It checks whether the chain was stopped between blocks.
func (c *Chain) pullAfterJoin() error {
	//TODO
	return errors.New("not implemented yet: pull after join block")
}
