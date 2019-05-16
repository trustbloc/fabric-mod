Peer Roles Enhancements
=======================

The peers are not split into two roles to enhance endorsments and commit
throughput in the Fabric network.

TODO additional comments

These roles are:

Endorser
--------

An endorser peer will only do endorsements of transactions and will never
right to the ledger, it will update its data from ledger periodically through
gossip service.

There can be multiple endorsers for a given org.

TODO additional comments

Committer
---------

A committer peer will deal with only committing transactions to the ledger.
This way all other peers are relieved from writing to the ledger and execute
a larger number of endorsements.

There should be only 1 endorser for a given org.

TODO additional comments

.. Licensed under the Apache License, Version 2.0 (Apache-2.0)
https://www.apache.org/licenses/LICENSE-2.0
